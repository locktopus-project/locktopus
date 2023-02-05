package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/locktopus-project/locktopus/internal/constants"
	ns "github.com/locktopus-project/locktopus/internal/namespace"
)

func statsV1Handler(w http.ResponseWriter, r *http.Request, abandonTimeout time.Duration) {
	nsParam := r.URL.Query().Get(constants.NamespaceQueryParameterName)

	if nsParam == "" {
		w.Write([]byte(fmt.Sprintf("URL parameter '%s' is required", constants.NamespaceQueryParameterName)))
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

	w.WriteHeader(http.StatusOK)
	w.Write(serialized)
}
