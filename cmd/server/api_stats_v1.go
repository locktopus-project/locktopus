package main

import (
	"encoding/json"
	"net/http"

	ns "github.com/xshkut/gearlock/internal/namespace"
)

func statsV1Handler(w http.ResponseWriter, r *http.Request) {
	nsParam := r.URL.Query().Get("namespace")

	if nsParam == "" {
		w.Write([]byte("URL parameter 'namespace' is required"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	namespace := ns.GetNamespaceStatistics(nsParam)
	if namespace == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Namespace not found"))
		return
	}

	serialized, err := json.Marshal(namespace)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Cannot serialize namespace statistics"))
		return
	}

	w.Write(serialized)
	w.WriteHeader(http.StatusOK)
}
