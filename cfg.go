package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Cfg struct {
	Url  string   `json:"url"`
	User string   `json:"user"`
	Pass string   `json:"pass"`
	Root string   `json:"root"`
	Mode string   `json:"mode"`
	Thr  int      `json:"thr"`
	Typ  string   `json:"typ"`
	List []string `json:"list"`
}

const mag = "CFG_TAIL1"
const maxT = int64(1 << 20) // 最多读最后 1MB

func rdCfg(exe string) (*Cfg, error) {
	f, err := os.Open(exe)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	sz := fi.Size()
	if sz <= int64(len(mag)+4) {
		return nil, fmt.Errorf("no cfg")
	}

	n := sz
	if n > maxT {
		n = maxT
	}
	buf := make([]byte, int(n))
	off := sz - n
	_, err = f.ReadAt(buf, off)
	if err != nil {
		return nil, err
	}

	idx := bytes.LastIndex(buf, []byte(mag))
	if idx < 0 {
		return nil, fmt.Errorf("cfg tag not found")
	}
	pos := idx + len(mag)
	if pos+4 > len(buf) {
		return nil, fmt.Errorf("cfg len miss")
	}
	ln := int(binary.BigEndian.Uint32(buf[pos : pos+4]))
	pos += 4
	if ln <= 0 || pos+ln > len(buf) {
		return nil, fmt.Errorf("cfg len bad")
	}
	js := buf[pos : pos+ln]

	var c Cfg
	if err := json.Unmarshal(js, &c); err != nil {
		return nil, err
	}
	if c.Url == "" {
		return nil, fmt.Errorf("cfg url empty")
	}
	if len(c.List) == 0 {
		return nil, fmt.Errorf("cfg list empty")
	}
	m := strings.ToLower(strings.TrimSpace(c.Mode))
	if m == "" {
		m = "over"
	}
	if m != "over" && m != "skip" {
		return nil, fmt.Errorf("cfg mode bad")
	}
	c.Mode = m
	return &c, nil
}
