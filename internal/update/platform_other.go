//go:build !windows

package update

import (
	"errors"

	"github.com/lalamefine/lattis/internal/msg"
)

// DetectKind : hors Windows, la nouvelle version est seulement signalée
// (page de la version) ; macOS et Linux suivront (étude, § 2).
func DetectKind(string) Kind { return KindOther }

// VerifySignature : Authenticode n'existe que sous Windows.
func VerifySignature(string) error { return errors.ErrUnsupported }

// SelfSigned : pas de contrôle de signature hors Windows.
func SelfSigned(string) bool { return false }

func apply(Pending, string, bool) error { return msg.New("update.unsupported", nil) }
