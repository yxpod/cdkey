package cdkey

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type statusError struct {
	Code     int    `json:"code"`
	HTTPCode int    `json:"-"`
	Msg      string `json:"msg"`
	Cmd      string `json:"cmd`
}

func (e statusError) Json() string {
	b, _ := json.Marshal(e)
	return string(b)
}

func (e statusError) Error() string {
	return fmt.Sprintf("%v:%v", e.Code, e.Msg)
}

func (e statusError) affix(v interface{}) statusError {
	e.Msg = fmt.Sprintf("%v, %v", e.Msg, v)
	return e
}

var (
	errBadRequest        = statusError{1001, http.StatusBadRequest, "bad request", ""}
	errPackNotFound      = statusError{1002, http.StatusNotFound, "pack not found", ""}
	errPackAlreadyExists = statusError{1003, http.StatusNotAcceptable, "pack already exists", ""}
	errPackDisabled      = statusError{1004, http.StatusNotAcceptable, "pack is disabled", ""}
	errKeyNotFound       = statusError{1005, http.StatusNotFound, "key not found", ""}
	errKeyUsed           = statusError{1006, http.StatusNotAcceptable, "key already used", ""}
	errInvalidPackName   = statusError{1007, http.StatusNotAcceptable, "invalid pack name", ""}
	errInvalidPrefix     = statusError{1008, http.StatusNotAcceptable, "invalid prefix, accept base32 only", ""}
	errKeylenTooShort    = statusError{1009, http.StatusNotAcceptable, "keylen too short, unable to generate", ""}

	errFailedCreateDB     = statusError{2001, http.StatusServiceUnavailable, "failed create db on file system", ""}
	errFailedLoadDB       = statusError{2002, http.StatusServiceUnavailable, "failed load db from file system", ""}
	errFailedLoadKeys     = statusError{2003, http.StatusServiceUnavailable, "failed load keys from db", ""}
	errFailedSaveKeys     = statusError{2004, http.StatusServiceUnavailable, "failed save keys to db", ""}
	errFailedLoadPackInfo = statusError{2005, http.StatusServiceUnavailable, "failed load pack info", ""}
	errFailedSavePackInfo = statusError{2006, http.StatusServiceUnavailable, "failed save pack info", ""}

	errInternal    = statusError{3001, http.StatusInternalServerError, "", ""}
	errPackClosing = statusError{3002, http.StatusServiceUnavailable, "pack is closing", ""}
)
