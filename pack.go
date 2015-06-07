package cdkey

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

type packStatus string

func (s packStatus) Ready() bool {
	return string(s) == "ready"
}

type keyStatus bool

func (s keyStatus) dbVal() []byte {
	if s {
		return []byte("R")
	} else {
		return []byte("U")
	}
}

func (s keyStatus) String() string {
	if s {
		return "Ready"
	} else {
		return "Used"
	}
}

func loadKeyStatus(b []byte) keyStatus {
	if string(b) == "R" {
		return keyStatus(true)
	} else {
		return keyStatus(false)
	}
}

type PackInfo struct {
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	KeyLen     int        `json:"keylen"`
	PackSize   int        `json:"packsize"`
	Status     packStatus `json:"status"`
	Note       string     `json:"note"`
	CreateTime time.Time  `json:"createTime"`
}

type Pack struct {
	info    PackInfo
	infoMtx sync.RWMutex

	path     string
	Name     string
	db       *leveldb.DB
	closeMtx sync.RWMutex
}

func LoadPack(path string) (*Pack, error) {
	info_logf("start load pack (path:%v)", path)

	b, err := ioutil.ReadFile(filepath.Join(path, "pack.json"))
	if err != nil {
		error_log(err)
		return nil, errFailedLoadPackInfo.affix(err)
	}

	var info PackInfo
	if err := json.Unmarshal(b, &info); err != nil {
		error_log(err)
		return nil, errFailedLoadPackInfo.affix(err)
	}

	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		error_log(err)
		return nil, errFailedLoadDB.affix(err)
	}

	info_logf("pack %v loaded", info.Name)

	return &Pack{
		info: info,
		path: path,
		Name: info.Name,
		db:   db,
	}, nil
}

func CreatePack(path string, name, prefix string, keylen, packsize int, note string) (*Pack, error) {
	if strings.ContainsAny(name, `<>:"/\|?*_`) {
		error_logf("invalid pack name: %v", name)
		return nil, errInvalidPackName.affix(fmt.Sprintf("name:%v", name))
	}

	prefix, ok := NormalizeKey(prefix)
	if !ok {
		error_logf("invalid pack prefix: %v", prefix)
		return nil, errInvalidPrefix.affix(fmt.Sprintf("prefix:%v", prefix))
	}

	info_logf("start generate keys (prefix:%v, keylen:%v, packsize:%v)", prefix, keylen, packsize)
	keys, err := KeyGenN(prefix, keylen, packsize)
	if err != nil {
		error_log(err)
		return nil, err
	}
	info_logf("keys generated (prefix:%v, keylen:%v, packsize:%v)", name, keylen, packsize)

	info := PackInfo{
		Name:       name,
		Prefix:     prefix,
		KeyLen:     keylen,
		PackSize:   packsize,
		Status:     packStatus("initial"),
		Note:       note,
		CreateTime: time.Now(),
	}

	fullPath := filepath.Join(path, name)

	info_logf("start create pack db (name:%v, path:%v)", name, fullPath)
	db, err := leveldb.OpenFile(fullPath, &opt.Options{ErrorIfExist: true})
	if err != nil {
		error_log(err)
		return nil, errFailedCreateDB.affix(err)
	}
	info_logf("pack db created (name:%v, path:%v)", name, fullPath)

	info_logf("start write keys to db (prefix:%v, keylen:%v, packsize:%v)", prefix, keylen, packsize)
	batch := &leveldb.Batch{}
	for _, k := range keys {
		batch.Put([]byte(k), keyStatus(true).dbVal())
	}

	if err := db.Write(batch, nil); err != nil {
		error_log("failed save keys: ", err)
		db.Close()
		os.RemoveAll(fullPath)
		return nil, errFailedSaveKeys.affix(err)
	}
	info_logf("finish write keys to db (prefix:%v, keylen:%v, packsize:%v)", prefix, keylen, packsize)

	p := &Pack{
		info: info,
		path: fullPath,
		Name: info.Name,
		db:   db,
	}

	if err := p.saveInfo(); err != nil {
		db.Close()
		os.RemoveAll(fullPath)
		return nil, err
	}

	return p, nil
}

func (p *Pack) saveInfo() error {
	info_logf("start save PackInfo (name:%v, path:%v)", p.Name, p.path)
	b, err := json.Marshal(p.info)
	if err != nil {
		error_logf("failed marshal PackInfo: %v", err)
		return errFailedSavePackInfo.affix(err)
	}
	err = ioutil.WriteFile(filepath.Join(p.path, "pack.json"), b, 0644)
	if err != nil {
		error_logf("failed save pack.json: %v", err)
		return errFailedSavePackInfo.affix(err)
	}
	info_logf("finish save PackInfo (name:%v, path:%v)", p.Name, p.path)
	return nil
}

func (p *Pack) Info() PackInfo {
	p.infoMtx.RLock()
	defer p.infoMtx.RUnlock()

	return p.info
}

func (p *Pack) Enable() error {
	p.infoMtx.Lock()
	defer p.infoMtx.Unlock()

	p.info.Status = packStatus("ready")
	return p.saveInfo()
}

func (p *Pack) Disable(msg string) error {
	p.infoMtx.Lock()
	defer p.infoMtx.Unlock()

	if msg == "ready" {
		msg = ""
	}
	p.info.Status = packStatus(msg)
	return p.saveInfo()
}

type KeyInfo struct {
	Key    string `json:"key"`
	Status string `json:"status"`
}

func (p *Pack) ListKeys() ([]KeyInfo, error) {
	p.closeMtx.RLock()
	defer p.closeMtx.RUnlock()

	if p.db == nil {
		return nil, errPackClosing
	}

	var ks []KeyInfo
	iter := p.db.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		ks = append(ks, KeyInfo{
			Key:    string(iter.Key()),
			Status: loadKeyStatus(iter.Value()).String(),
		})
	}

	if iter.Error() != nil {
		error_log(iter.Error())
		return nil, errFailedLoadKeys.affix(iter.Error())
	}

	info_logf("list keys (pack:%v, packsize:%v)", p.Name, len(ks))

	return ks, nil
}

func (p *Pack) UseKey(key string) error {
	p.closeMtx.RLock()
	defer p.closeMtx.RUnlock()

	if p.db == nil {
		return errPackClosing
	}

	p.infoMtx.RLock()
	defer p.infoMtx.RUnlock()

	if !p.info.Status.Ready() {
		info_logf("pack is disabled (pack:%v, msg:%v)", p.Name, p.info.Status)
		return errPackDisabled.affix(fmt.Sprintf("msg:%v", p.info.Status))
	}

	key, _ = NormalizeKey(key)

	b, err := p.db.Get([]byte(key), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			info_logf("key not found (pack:%v, key:%v)", p.Name, key)
			return errKeyNotFound.affix(fmt.Sprintf("key:%v", key))
		} else {
			error_log(err)
			return errFailedLoadKeys.affix(err)
		}
	}

	status := loadKeyStatus(b)

	if !status {
		return errKeyUsed.affix(fmt.Sprintf("key:%v", key))
	}

	if err := p.db.Put([]byte(key), keyStatus(false).dbVal(), nil); err != nil {
		error_log(err)
		return errFailedSaveKeys.affix(err)
	}

	info_logf("key use (pack:%v, key:%v)", p.Name, key)
	return nil
}

func (p *Pack) Close() {
	p.closeMtx.Lock()
	defer p.closeMtx.Unlock()

	info_logf("start close pack (name:%v)", p.Name)

	p.db.Close()
	p.db = nil

	info_logf("pack closed (name:%v)", p.Name)
}
