package archive

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"runtime"
	"sync"

	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/zstd"
)

// Compression de la base dans l'archive (décision 0015,
// docs/format-lattis.md). La base est écrite en Zstandard rapide : une base
// SQLite s'y réduit de 2,5 à 9 fois, et l'archive s'écrit et se relit vite
// parce que la base est coupée en blocs compressés sur plusieurs cœurs.
// Les archives Store (avant la compression) et Deflate se lisent toujours.

// Zstd est la méthode ZIP 93 (Zstandard, APPNOTE 6.3.7) : lue par 7-Zip et
// WinRAR, pas par l'Explorateur de Windows ni l'Utilitaire d'archive de macOS.
const Zstd uint16 = 93

// Method est la méthode de compression de la base à l'enregistrement : Zstd
// (défaut), zip.Deflate ou zip.Store. Les trois se lisent toujours.
var Method = Zstd

const (
	// zstdBlock : taille d'une trame Zstandard. Des trames indépendantes
	// mises bout à bout forment un flux Zstandard ordinaire ; à 2 Mio, la
	// perte de taux face à un flux unique est sous le pour cent.
	zstdBlock = 2 << 20
	// deflateBlock : un bloc Deflate se termine par un vidage (bloc vide non
	// final) ; leur suite, close par un bloc final vide, est un flux Deflate
	// ordinaire.
	deflateBlock = 1 << 20
	// maxBlocks : blocs en cours au plus (mémoire : deux fois maxBlocks blocs).
	maxBlocks = 8
)

// parallel compresse par blocs indépendants, sur plusieurs cœurs, et les
// écrit dans l'ordre.
type parallel struct {
	w       io.Writer
	size    int
	encode  func(block []byte) []byte
	trailer []byte
	buf     []byte
	queue   chan chan []byte
	done    chan struct{}
	mu      sync.Mutex
	err     error
}

func newParallel(w io.Writer, size int, encode func([]byte) []byte, trailer []byte) *parallel {
	n := min(runtime.GOMAXPROCS(0), maxBlocks)
	p := &parallel{w: w, size: size, encode: encode, trailer: trailer,
		queue: make(chan chan []byte, n), done: make(chan struct{})}
	go p.drain(p.queue)
	return p
}

func (p *parallel) failed() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

func (p *parallel) drain(queue chan chan []byte) {
	defer close(p.done)
	for res := range queue {
		b := <-res
		if p.failed() != nil {
			continue
		}
		if _, err := p.w.Write(b); err != nil {
			p.mu.Lock()
			p.err = err
			p.mu.Unlock()
		}
	}
}

func (p *parallel) send(block []byte) {
	res := make(chan []byte, 1)
	p.queue <- res // attend quand maxBlocks blocs sont en cours
	go func() { res <- p.encode(block) }()
}

func (p *parallel) Write(b []byte) (int, error) {
	if p.queue == nil {
		return 0, errors.New("flux déjà fermé")
	}
	if err := p.failed(); err != nil {
		return 0, err
	}
	n := len(b)
	for len(b) > 0 {
		if p.buf == nil {
			p.buf = make([]byte, 0, p.size)
		}
		k := min(len(b), p.size-len(p.buf))
		p.buf = append(p.buf, b[:k]...)
		b = b[k:]
		if len(p.buf) == p.size {
			p.send(p.buf)
			p.buf = nil
		}
	}
	return n, nil
}

func (p *parallel) Close() error {
	if p.queue == nil {
		return errors.New("flux déjà fermé")
	}
	if len(p.buf) > 0 {
		p.send(p.buf)
		p.buf = nil
	}
	close(p.queue)
	p.queue = nil
	<-p.done
	if err := p.failed(); err != nil {
		return err
	}
	if len(p.trailer) == 0 {
		return nil
	}
	_, err := p.w.Write(p.trailer)
	return err
}

var flatePool = sync.Pool{New: func() any {
	w, _ := flate.NewWriter(nil, flate.BestSpeed)
	return w
}}

// finalBlock : bloc Deflate final vide (codes fixes, fin de bloc).
var finalBlock = []byte{0x03, 0x00}

// newDeflate : Deflate rapide (niveau 1 de klauspost/compress) par blocs.
func newDeflate(w io.Writer) (io.WriteCloser, error) {
	return newParallel(w, deflateBlock, func(block []byte) []byte {
		var out bytes.Buffer
		out.Grow(len(block)/2 + 1024)
		fw := flatePool.Get().(*flate.Writer)
		fw.Reset(&out)
		fw.Write(block) // dans un bytes.Buffer : sans erreur
		fw.Flush()
		flatePool.Put(fw)
		return out.Bytes()
	}, finalBlock), nil
}

// zstdEnc : encodeur partagé (EncodeAll se partage entre goroutines).
var zstdEnc = sync.OnceValue(func() *zstd.Encoder {
	e, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest),
		zstd.WithEncoderConcurrency(min(runtime.GOMAXPROCS(0), maxBlocks)))
	if err != nil {
		panic(err) // options constantes
	}
	return e
})

// newZstd : Zstandard au plus rapide, une trame par bloc.
func newZstd(w io.Writer) (io.WriteCloser, error) {
	enc := zstdEnc()
	return newParallel(w, zstdBlock, func(block []byte) []byte {
		return enc.EncodeAll(block, make([]byte, 0, len(block)/2+1024))
	}, nil), nil
}

func newZstdReader(r io.Reader) io.ReadCloser {
	// Fenêtre bornée : Lattis écrit des trames de 2 Mio ; 128 Mio laisse
	// passer une base recompressée par 7-Zip sans laisser une archive
	// forgée réserver des centaines de mégaoctets.
	d, err := zstd.NewReader(r, zstd.WithDecoderMaxWindow(128<<20))
	if err != nil {
		return io.NopCloser(errReader{err})
	}
	return d.IOReadCloser()
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

// newZipWriter : les méthodes d'écriture de Lattis (Zstandard et Deflate
// par blocs en parallèle).
func newZipWriter(w io.Writer) *zip.Writer {
	zw := zip.NewWriter(w)
	zw.RegisterCompressor(zip.Deflate, newDeflate)
	zw.RegisterCompressor(Zstd, newZstd)
	return zw
}

// OpenZip ouvre une archive en lisant Store, Deflate (décodeur plus rapide
// que celui de la bibliothèque standard) et Zstandard.
func OpenZip(path string) (*zip.ReadCloser, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	zr.RegisterDecompressor(zip.Deflate, flate.NewReader)
	zr.RegisterDecompressor(Zstd, newZstdReader)
	return zr, nil
}
