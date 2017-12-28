package main

import (
	"log"

	webhooks "gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/github"
)

func (p *purger) handleDelete(payload interface{}, header webhooks.Header) {
	deletePayload, ok := payload.(github.DeletePayload)
	if !ok {
		log.Println("Got unexpected payload:", payload)
		return
	}
	if deletePayload.RefType != "branch" {
		log.Println("Ignoring non branch payload:", payload)
		return
	}
	if err := p.purge(deletePayload.Repository.FullName, deletePayload.Ref); err != nil {
		// FIXME: We should return a failure to the webhook or retry
		log.Println("Couldn't purge namespace:", err)
	}
}
