package domain

import (
	"io"
	"regexp"
	"time"
)

// Image es una imagen del catálogo subida desde el Admin. Content se debe cerrar.
type Image struct {
	Content     io.ReadCloser
	Size        int64
	ContentType string
	UploadedAt  time.Time
}

// Los nombres los genera el servidor (UnixNano + extensión): cualquier otra
// cosa en /uploads/ se rechaza sin consultar la base.
var imageNameRe = regexp.MustCompile(`^[0-9]{1,25}\.(jpg|png|webp)$`)

// ValidImageName indica si el nombre tiene el formato que genera el servidor.
func ValidImageName(name string) bool { return imageNameRe.MatchString(name) }
