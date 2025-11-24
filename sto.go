package main

import (
	"context"
	"strings"
)

type Sto interface {
	Put(ctx context.Context, loc, rem string, sz int64) error
	Has(ctx context.Context, rem string) (bool, error)
	Mk(ctx context.Context, dir string) error
	Cls()
}

func mkSto(cfg *Cfg) (Sto, error) {
	typ := strings.ToLower(strings.TrimSpace(cfg.Typ))
	if typ == "" {
		u := strings.ToLower(cfg.Url)
		switch {
		case strings.HasPrefix(u, "ftp://"):
			typ = "ftp"
		case strings.HasPrefix(u, "smb://"), strings.HasPrefix(u, `\\`):
			typ = "smb"
		default:
			typ = "dav"
		}
	}

	switch typ {
	case "ftp":
		return newFtp(cfg)
	case "smb":
		return newSmb(cfg)
	default:
		return newDav(cfg)
	}
}
