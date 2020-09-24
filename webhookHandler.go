package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xmidt-org/argus/chrysom"
	"github.com/xmidt-org/argus/model"
	"github.com/xmidt-org/webpa-common/webhook"
	"io/ioutil"
	"net/http"
)

type Registry struct {
	hookStore *chrysom.Client
	config    chrysom.ClientConfig
}

func NewRegistry(config chrysom.ClientConfig) (*Registry, error) {
	argus, err := chrysom.CreateClient(config)
	fmt.Println(config)
	if err != nil {
		return nil, err
	}
	err = argus.Start(context.Background())
	if err != nil {
		return nil, err
	}

	return &Registry{
		hookStore: argus,
		config:    config,
	}, nil
}

// jsonResponse is an internal convenience function to write a json response
func jsonResponse(rw http.ResponseWriter, code int, msg string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(code)
	rw.Write([]byte(fmt.Sprintf(`{"message":"%s"}`, msg)))
}

// update is an api call to processes a listener registration for adding and updating
func (r *Registry) UpdateRegistry(rw http.ResponseWriter, req *http.Request) {
	payload, err := ioutil.ReadAll(req.Body)

	w, err := webhook.NewW(payload, req.RemoteAddr)
	if err != nil {
		jsonResponse(rw, http.StatusBadRequest, err.Error())
		return
	}
	webhook := map[string]interface{}{}
	data, err := json.Marshal(&w)
	if err != nil {
		// this should never happen
		jsonResponse(rw, http.StatusInternalServerError, err.Error())
		return
	}
	err = json.Unmarshal(data, &webhook)
	if err != nil {
		// this should never happen
		jsonResponse(rw, http.StatusInternalServerError, err.Error())
		return
	}

	_, err = r.hookStore.Push(model.Item{
		Identifier: w.ID(),
		Data:       webhook,
		TTL:        r.config.DefaultTTL,
	}, "")
	if err != nil {
		jsonResponse(rw, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(rw, http.StatusOK, "Success")
}

func (r *Registry) Stop(ctx context.Context) error {
	return r.hookStore.Stop(ctx)
}

func convertItemToWebhook(item model.Item) (webhook.W, error) {
	hook := webhook.W{}
	tempBytes, err := json.Marshal(&item.Data)
	if err != nil {
		return hook, err
	}
	err = json.Unmarshal(tempBytes, &hook)
	if err != nil {
		return hook, err
	}
	return hook, nil
}
