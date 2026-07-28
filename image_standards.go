package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strings"
)

type uploadedImageInfo struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
}

func inspectUploadedImage(data []byte, ext string) (uploadedImageInfo, error) {
	if len(data) == 0 {
		return uploadedImageInfo{}, errors.New("el archivo de imagen está vacío")
	}
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == ".webp" {
		w, h, ok := decodeWebPSize(data)
		if !ok || w <= 0 || h <= 0 {
			return uploadedImageInfo{}, errors.New("no fue posible leer las dimensiones del archivo WEBP")
		}
		return uploadedImageInfo{Width: w, Height: h, Format: "webp"}, nil
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return uploadedImageInfo{}, errors.New("el archivo no contiene una imagen válida")
	}
	return uploadedImageInfo{Width: cfg.Width, Height: cfg.Height, Format: format}, nil
}

func decodeWebPSize(data []byte) (int, int, bool) {
	if len(data) < 30 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return 0, 0, false
	}
	switch string(data[12:16]) {
	case "VP8X":
		w := 1 + int(data[24]) + int(data[25])<<8 + int(data[26])<<16
		h := 1 + int(data[27]) + int(data[28])<<8 + int(data[29])<<16
		return w, h, w > 0 && h > 0
	case "VP8L":
		if len(data) < 25 || data[20] != 0x2f {
			return 0, 0, false
		}
		b0, b1, b2, b3 := data[21], data[22], data[23], data[24]
		w := 1 + int(b0) + (int(b1&0x3f) << 8)
		h := 1 + int(b1>>6) + (int(b2) << 2) + (int(b3&0x0f) << 10)
		return w, h, w > 0 && h > 0
	case "VP8 ":
		if len(data) < 30 || data[23] != 0x9d || data[24] != 0x01 || data[25] != 0x2a {
			return 0, 0, false
		}
		w := int(data[26]) | int(data[27])<<8
		h := int(data[28]) | int(data[29])<<8
		w &= 0x3fff
		h &= 0x3fff
		return w, h, w > 0 && h > 0
	}
	return 0, 0, false
}

func imageRatioWithin(width, height int, target, tolerance float64) bool {
	if width <= 0 || height <= 0 || target <= 0 {
		return false
	}
	ratio := float64(width) / float64(height)
	return math.Abs(ratio-target)/target <= tolerance
}

func validateCatalogImageStandard(info uploadedImageInfo) error {
	if info.Width < 800 || info.Height < 800 {
		return fmt.Errorf("la imagen mide %d × %d px; el catálogo requiere mínimo 800 × 800 px y recomienda 1200 × 1200 px", info.Width, info.Height)
	}
	if !imageRatioWithin(info.Width, info.Height, 1, 0.05) {
		return fmt.Errorf("la imagen mide %d × %d px; para catálogo y envío por WhatsApp debe ser cuadrada (relación 1:1), idealmente 1200 × 1200 px", info.Width, info.Height)
	}
	return nil
}

func validateLandingHeroStandard(info uploadedImageInfo) error {
	if info.Width < 1200 || info.Height < 900 {
		return fmt.Errorf("la imagen mide %d × %d px; la portada de landing requiere mínimo 1200 × 900 px y recomienda 1600 × 1200 px", info.Width, info.Height)
	}
	if !imageRatioWithin(info.Width, info.Height, 4.0/3.0, 0.05) {
		return fmt.Errorf("la imagen mide %d × %d px; la portada de landing debe usar relación 4:3, idealmente 1600 × 1200 px", info.Width, info.Height)
	}
	return nil
}
