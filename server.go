package cdkey

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type Server struct {
	path  string
	packs map[string]*Pack

	mtx sync.RWMutex
}

func NewServer(path string) *Server {
	info_logf("new CDKeyServer (path:%v)", path)

	if f, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			info_logf("path %v not exists, try to create", path)

			if err := os.MkdirAll(path, 0644); err != nil {
				error_logf("failed mkdir (path:%v)", path)
				return nil
			}

			info_logf("created path %v", path)
		} else {
			error_logf("failed access path %v", path)
			return nil
		}
	} else {
		if !f.IsDir() {
			error_logf("%v is not a directory", path)
			return nil
		}
	}

	packs := make(map[string]*Pack)
	filepath.Walk(path, func(packPath string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			return nil
		}

		if filepath.Clean(filepath.Join(packPath, "..")) != filepath.Clean(path) {
			return nil
		}

		if f, err := os.Stat(filepath.Join(packPath, "pack.json")); err != nil || f.IsDir() {
			return nil
		}

		if p, err := LoadPack(packPath); err == nil {
			packs[p.Name] = p
		}

		return nil
	})

	return &Server{
		path:  path,
		packs: packs,
	}
}

func (s *Server) ListPacks() []PackInfo {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	var packs []PackInfo
	for _, p := range s.packs {
		packs = append(packs, p.Info())
	}
	return packs
}

func (s *Server) AddPack(name string, prefix string, keylen, packsize int, note string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if _, ok := s.packs[name]; ok {
		error_logf("pack already exists (name:%v)", name)
		return errPackAlreadyExists.affix(fmt.Sprintf("name:%v", name))
	}

	p, err := CreatePack(s.path, name, prefix, keylen, packsize, note)
	if err != nil {
		return err
	}

	s.packs[name] = p
	return nil
}

func (s *Server) RemovePack(name string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if p, ok := s.packs[name]; !ok {
		error_logf("pack not found (name:%v)", name)
		return errPackNotFound.affix(fmt.Sprintf("name:%v", name))
	} else {
		p.Close()
		os.RemoveAll(filepath.Join(s.path, name))
		delete(s.packs, name)

		info_logf("pack removed (name:%v)", name)
		return nil
	}
}

func (s *Server) EnablePack(name string) error {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	if p, ok := s.packs[name]; ok {
		return p.Enable()
	} else {
		error_logf("pack not found (name:%v)", name)
		return errPackNotFound.affix(fmt.Sprintf("name:%v", name))
	}
}

func (s *Server) DisablePack(name string, msg string) error {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	if p, ok := s.packs[name]; ok {
		return p.Disable(msg)
	} else {
		error_logf("pack not found (name:%v)", name)
		return errPackNotFound.affix(fmt.Sprintf("name:%v", name))
	}
}

func (s *Server) ListKeys(packName string) ([]KeyInfo, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	if p, ok := s.packs[packName]; ok {
		return p.ListKeys()
	} else {
		error_logf("pack not found (name:%v)", packName)
		return nil, errPackNotFound.affix(fmt.Sprintf("name:%v", packName))
	}
}

func (s *Server) UseKey(packName, key string) error {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	if p, ok := s.packs[packName]; ok {
		return p.UseKey(key)
	} else {
		error_logf("pack not found (name:%v)", packName)
		return errPackNotFound.affix(fmt.Sprintf("name:%v", packName))
	}
}

func (s *Server) Stop() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for _, p := range s.packs {
		p.Close()
	}
}

func (s *Server) HTTPServeMux() *http.ServeMux {
	m := http.NewServeMux()

	m.HandleFunc("/pack.list", s.handlePackList)
	m.HandleFunc("/pack.add", s.handlePackAdd)
	m.HandleFunc("/pack.remove", s.handlePackRemove)
	m.HandleFunc("/pack.enable", s.handlePackEnable)
	m.HandleFunc("/pack.disable", s.handlePackDisable)

	m.HandleFunc("/key.list", s.handleKeyList)
	m.HandleFunc("/key.use", s.handleKeyUse)

	return m
}

func putStatusError(w http.ResponseWriter, cmd string, err error) {
	if e, ok := err.(statusError); ok {
		e.Cmd = cmd
		http.Error(w, e.Json(), e.HTTPCode)
	} else {
		putStatusError(w, cmd, errInternal.affix(err))
	}
}

func readJsonRequest(r *http.Request, v interface{}) error {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(b, v); err != nil {
		return errBadRequest.affix(err)
	}

	return nil
}

func (s *Server) handlePackList(w http.ResponseWriter, r *http.Request) {
	rsp, _ := json.Marshal(struct {
		Cmd   string     `json:"cmd"`
		Packs []PackInfo `json:"packs"`
	}{
		Cmd:   "pack.list",
		Packs: s.ListPacks(),
	})

	w.WriteHeader(http.StatusOK)
	w.Write(rsp)
}

func (s *Server) handlePackAdd(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Name     string `json:"name"`
		Prefix   string `json:"prefix"`
		KeyLen   int    `json:"keylen"`
		PackSize int    `json:"packsize"`
		Note     string `json:"note"`
	}{}

	if err := readJsonRequest(r, &req); err != nil {
		putStatusError(w, "pack.add", err)
		return
	}

	if err := s.AddPack(req.Name, req.Prefix, req.KeyLen, req.PackSize, req.Note); err != nil {
		putStatusError(w, "pack.add", err)
		return
	}

	rsp, _ := json.Marshal(struct {
		Cmd  string `json:"cmd"`
		Pack string `json:"pack"`
	}{
		Cmd:  "pack.add",
		Pack: req.Name,
	})

	w.WriteHeader(http.StatusOK)
	w.Write(rsp)
}

func (s *Server) handlePackRemove(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Pack string `json:"pack"`
	}{}

	if err := readJsonRequest(r, &req); err != nil {
		putStatusError(w, "pack.remove", err)
		return
	}

	if err := s.RemovePack(req.Pack); err != nil {
		putStatusError(w, "pack.remove", err)
		return
	}

	rsp, _ := json.Marshal(struct {
		Cmd  string `json:"cmd"`
		Pack string `json:"pack"`
	}{
		Cmd:  "pack.add",
		Pack: req.Pack,
	})

	w.WriteHeader(http.StatusOK)
	w.Write(rsp)
}

func (s *Server) handlePackEnable(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Pack string `json:"pack"`
	}{}

	if err := readJsonRequest(r, &req); err != nil {
		putStatusError(w, "pack.enable", err)
		return
	}

	if err := s.EnablePack(req.Pack); err != nil {
		putStatusError(w, "pack.enable", err)
		return
	}

	rsp, _ := json.Marshal(struct {
		Cmd  string `json:"cmd"`
		Pack string `json:"pack"`
	}{
		Cmd:  "pack.enable",
		Pack: req.Pack,
	})

	w.WriteHeader(http.StatusOK)
	w.Write(rsp)
}

func (s *Server) handlePackDisable(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Pack string `json:"pack"`
		Msg  string `json:"msg"`
	}{}

	if err := readJsonRequest(r, &req); err != nil {
		putStatusError(w, "pack.disable", err)
		return
	}

	if err := s.DisablePack(req.Pack, req.Msg); err != nil {
		putStatusError(w, "pack.disable", err)
		return
	}

	rsp, _ := json.Marshal(struct {
		Cmd  string `json:"cmd"`
		Pack string `json:"pack"`
	}{
		Cmd:  "pack.disable",
		Pack: req.Pack,
	})

	w.WriteHeader(http.StatusOK)
	w.Write(rsp)
}

func (s *Server) handleKeyList(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Pack string `json:"pack"`
	}{}

	if err := readJsonRequest(r, &req); err != nil {
		putStatusError(w, "key.list", err)
		return
	}

	keys, err := s.ListKeys(req.Pack)
	if err != nil {
		putStatusError(w, "key.list", err)
		return
	}

	rsp, _ := json.Marshal(struct {
		Cmd  string    `json:"cmd"`
		Pack string    `json:"pack"`
		Keys []KeyInfo `json:"keys"`
	}{
		Cmd:  "key.list",
		Pack: req.Pack,
		Keys: keys,
	})

	w.WriteHeader(http.StatusOK)
	w.Write(rsp)
}

func (s *Server) handleKeyUse(w http.ResponseWriter, r *http.Request) {

	req := struct {
		Pack string `json:"pack`
		Key  string `json:"key`
	}{}

	if err := readJsonRequest(r, &req); err != nil {
		putStatusError(w, "key.use", err)
		return
	}

	if err := s.UseKey(req.Pack, req.Key); err != nil {
		putStatusError(w, "key.use", err)
		return
	}

	rsp, _ := json.Marshal(struct {
		Cmd  string `json:"cmd"`
		Pack string `json:"pack"`
		Key  string `json:"key"`
	}{
		Cmd:  "key.use",
		Pack: req.Pack,
		Key:  req.Key,
	})

	w.WriteHeader(http.StatusOK)
	w.Write(rsp)
}
