package main

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
)

var (
	formTmpl = template.Must(template.ParseFiles("form.html"))
	mapTmpl  = template.Must(template.ParseFiles("map.html"))
)

type People struct {
	Seated, Floored int
}

type Snapshot struct {
	People
	Decibels  int
	Area      string
	TimeStamp time.Time
}

func extractNumbers(r *http.Request, fields []string) (map[string]int, appengine.MultiError) {
	var err error
	results := map[string]int{}
	var badness appengine.MultiError
	for _, n := range fields {
		if results[n], err = strconv.Atoi(r.FormValue(n)); err != nil {
			badness = append(badness, fmt.Errorf("bad value for \"%s\" field: %v", n, err))
		}
	}
	return results, badness
}

func reportError(ctx context.Context, statusCode int, msg string, w http.ResponseWriter) {
	log.Errorf(ctx, msg)
	w.WriteHeader(statusCode)
	fmt.Fprintln(w, msg)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	ctx := appengine.NewContext(r)

	area := r.FormValue("area")
	if r.Method == http.MethodGet && len(area) > 0 {
		formTmpl.Execute(w, struct{ Area string }{Area: area})
		return
	}
	floor := r.FormValue("floor")

	if r.Method == http.MethodPost {
		if len(area) == 0 {
			reportError(ctx, http.StatusBadRequest, "Hidden form field \"area\" not provided.", w)
		}
		fields := []string{"seated", "floored"}
		if len(r.FormValue("decibels")) > 0 {
			fields = append(fields, "decibels")
		}
		values, badness := extractNumbers(r, fields)
		if len(badness) != 0 {
			msg := fmt.Sprintf("Failure parsing numbers: %v.", badness)
			reportError(ctx, http.StatusBadRequest, msg, w)
			return
		}
		record := Snapshot{
			TimeStamp: time.Now(),
			Area:      area,
			People: People{
				Seated:  values["seated"],
				Floored: values["floored"],
			},
			Decibels: values["decibels"],
		}
		key := datastore.NewIncompleteKey(ctx, "Snapshot", nil)
		if _, err := datastore.Put(ctx, key, &record); err != nil {
			msg := fmt.Sprintf("Unable to create new record in DB: %v.", err)
			reportError(ctx, http.StatusInternalServerError, msg, w)
			return
		}
		if area[0] == 'U' {
			floor = "2"
		}
	}

	if floor == "" {
		floor = "1"
	}
	if floor != "1" && floor != "2" {
		msg := fmt.Sprintf("Invalid floor: %v.", floor)
		reportError(ctx, http.StatusBadRequest, msg, w)
		return
	}
	mapTmpl.Execute(w, struct{ Floor string }{Floor: floor})
}

func main() {
	http.HandleFunc("/", indexHandler)
	appengine.Main()
}
