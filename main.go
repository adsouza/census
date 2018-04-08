package main

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"

	"github.com/hashicorp/go-multierror"
)

var (
	indexTmpl = template.Must(template.ParseFiles("index.html"))
)

type People struct {
	Total, Grouped, Solitary, Asleep int
}

type Snapshot struct {
	People
	Decibels, Laptops int
}

func reportError(ctx context.Context, msg string, w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	log.Errorf(ctx, msg)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if r.Method == http.MethodPost {
		ctx := appengine.NewContext(r)
		record := Snapshot{}
		var badness *multierror.Error
		var err error
		if record.Total, err = strconv.Atoi(r.FormValue("total")); err != nil {
			badness = multierror.Append(badness, err)
		}
		if record.Grouped, err = strconv.Atoi(r.FormValue("grouped")); err != nil {
			badness = multierror.Append(badness, err)
		}
		if record.Solitary, err = strconv.Atoi(r.FormValue("solitary")); err != nil {
			badness = multierror.Append(badness, err)
		}
		if record.Asleep, err = strconv.Atoi(r.FormValue("asleep")); err != nil {
			badness = multierror.Append(badness, err)
		}
		if badness.ErrorOrNil() != nil {
			msg := fmt.Sprintf("Failure parsing numbers: %v.", badness)
			reportError(ctx, msg, w)
			return
		}
		if record.Total != record.Grouped+record.Solitary+record.Asleep {
			msg := fmt.Sprintf("Total (%d) != grouped (%d) + solitary (%d) + asleep (%d).",
				record.Total, record.Grouped, record.Solitary, record.Asleep)
			reportError(ctx, msg, w)
			return
		}
	}

	indexTmpl.Execute(w, nil)
}

func main() {
	http.HandleFunc("/", indexHandler)
	appengine.Main()
}
