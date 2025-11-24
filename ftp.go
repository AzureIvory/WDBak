package main

import (
    "context"
    "log"
    "net/url"
    "os"
    "path"
    "strings"
    "time"

    ftp "github.com/jlaffaye/ftp"
)

type FtpSto struct {
    con  *ftp.ServerConn
    base string
}

func newFtp(cfg *Cfg) (Sto, error) {
    u, err := url.Parse(cfg.Url)
    if err != nil {
        return nil, err
    }
    h := u.Host
    if h == "" {
        h = cfg.Url
    }
    if !strings.Contains(h, ":") {
        h = h + ":21"
    }

    con, err := ftp.Dial(h, ftp.DialWithTimeout(30*time.Second))
    if err != nil {
        return nil, err
    }

    usr := cfg.User
    pas := cfg.Pass
    if usr == "" {
        usr = "anonymous"
        pas = "anonymous"
    }
    if err := con.Login(usr, pas); err != nil {
        _ = con.Quit()
        return nil, err
    }

    bp := strings.Trim(u.Path, "/")
    return &FtpSto{
        con:  con,
        base: bp,
    }, nil
}

func (f *FtpSto) Cls() {
    if f.con != nil {
        _ = f.con.Quit()
    }
}

func (f *FtpSto) full(rem string) string {
    if f.base == "" {
        return rem
    }
    return path.Join(f.base, rem)
}

func (f *FtpSto) Mk(ctx context.Context, dir string) error {
    if dir == "" {
        return nil
    }
    p := f.full(dir)
    if err := f.con.MakeDir(p); err != nil {
        s := strings.ToLower(err.Error())
        if strings.Contains(s, "exist") {
            return nil
        }
        return err
    }
    return nil
}

func (f *FtpSto) Has(ctx context.Context, rem string) (bool, error) {
    p := f.full(rem)
    _, err := f.con.FileSize(p)
    if err == nil {
        return true, nil
    }
    if isFNF(err) {
        return false, nil
    }
    return false, err
}

func (f *FtpSto) Put(ctx context.Context, loc, rem string, sz int64) error {
    p := f.full(rem)
    fh, err := os.Open(loc)
    if err != nil {
        return err
    }
    defer fh.Close()

    t0 := time.Now()
    if err := f.con.Stor(p, fh); err != nil {
        return err
    }
    dur := time.Since(t0).Seconds()
    if dur <= 0 {
        dur = 0.001
    }
    mb := float64(sz) / 1024.0 / 1024.0
    spd := mb / dur
    log.Printf("[OK ] %s -> ftp:%s (%.2f MB, %.1fs, %.2f MB/s)\n",
        loc, p, mb, dur, spd)
    return nil
}

func isFNF(err error) bool {
    if err == nil {
        return false
    }
    s := strings.ToLower(err.Error())
    if strings.Contains(s, "550") ||
        strings.Contains(s, "not found") ||
        strings.Contains(s, "no such file") {
        return true
    }
    return false
}
