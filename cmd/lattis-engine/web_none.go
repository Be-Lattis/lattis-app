//go:build !embedweb

package main

import "io/fs"

// Sans -tags embedweb : pas de frontend embarqué (--static, ou lattisd le
// sert lui-même).
func embeddedWeb() fs.FS { return nil }
