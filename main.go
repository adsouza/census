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

func extractNumbers(r *http.Request, fields []string) (map[string]int, *multierror.Error) {
	var err error
	results := map[string]int{}
	var badness *multierror.Error
	for _, n := range fields {
		if results[n], err = strconv.Atoi(r.FormValue(n)); err != nil {
			badness = multierror.Append(badness, err)
		}
	}
	return results, badness
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
		var badness *multierror.Error
		fields := []string{"total", "grouped", "solitary", "asleep"}
		values, badness := extractNumbers(r, fields)
		if badness.ErrorOrNil() != nil {
			msg := fmt.Sprintf("Failure parsing numbers: %v.", badness)
			reportError(ctx, msg, w)
			return
		}
		record := Snapshot{
			People: People{
				Total:    values["total"],
				Grouped:  values["grouped"],
				Solitary: values["solitary"],
				Asleep:   values["asleep"],
			},
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
