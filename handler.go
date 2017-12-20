// Copyright 2016 Eleme Inc. All rights reserved.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Eagle-X/witch/system"
	"github.com/lszlsgslz/checking"
	"github.com/martini-contrib/render"
)

var (
	// ErrServerError is internal server error.
	ErrServerError = errors.New("Internal Server Error")
	// ErrBadRequest is bad request error.
	ErrBadRequest = errors.New("Bad Request")
)

func sysAction(control *system.Controller, req *http.Request, r render.Render) {
	bs, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Printf("Read request body error: %s", err)
		r.JSON(http.StatusInternalServerError, ErrServerError)
		return
	}
	log.Printf("Request action: %s", bs)
	action := &system.Action{}
	if err := json.Unmarshal(bs, action); err != nil {
		log.Printf("Invalid action format: %s", err)
		r.JSON(http.StatusBadRequest, ErrBadRequest)
		return
	}
	r.JSON(http.StatusOK, control.Handle(action))
}

func check(req *http.Request, r render.Render) {
	req.ParseForm()

	checkURL := req.FormValue("url")
	task := checking.NewCheckTask(checkURL)

	err := task.Check()
	if err != nil {
		r.JSON(http.StatusOK, system.ActionStatus{false, fmt.Sprintf("noPass: %s", task.Reason.Error())})
		return
	}

	r.JSON(http.StatusOK, system.ActionStatus{true, "pass"})
}
