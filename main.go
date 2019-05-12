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
	formTmpl    = template.Must(template.ParseFiles("form.html"))
	mapTmpl     = template.Must(template.ParseFiles("map.html"))
	historyTmpl = template.Must(template.ParseFiles("history.html"))
)

type IntFieldName string

const (
	id        IntFieldName = "id"
	timestamp IntFieldName = "ts"
	decibels  IntFieldName = "decibels"
	seated    IntFieldName = "seated"
	floored   IntFieldName = "floored"
)

type People struct {
	Seated, Floored int8
}

type Snapshot struct {
	People
	Decibels  int8
	Area      string
	TimeStamp time.Time
	//Key *datastore.Key `datastore:"__key__"`
}

func extractNumbers(r *http.Request, fields []IntFieldName) (map[IntFieldName]int, appengine.MultiError) {
	var err error
	results := map[IntFieldName]int{}
	var badness appengine.MultiError
	for _, n := range fields {
		if results[n], err = strconv.Atoi(r.FormValue(string(n))); err != nil {
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
	if r.Method == http.MethodGet && area != "" {
		formTmpl.Execute(w, struct{ Area string }{Area: area})
		return
	}
	floor := r.FormValue("floor")

	if r.Method == http.MethodPost {
		if area == "" {
			reportError(ctx, http.StatusBadRequest, "Hidden form field \"area\" not provided.", w)
		}
		fields := []IntFieldName{seated, floored}
		if len(r.FormValue("decibels")) > 0 {
			fields = append(fields, decibels)
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
				Seated:  int8(values[seated]),
				Floored: int8(values[floored]),
			},
			Decibels: int8(values[decibels]),
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

type KeyedRecord struct {
	Snapshot
	Key *datastore.Key
}

type Listing struct {
	Area    string
	Records []KeyedRecord
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	area := r.FormValue("area")
	if area == "" {
		reportError(ctx, http.StatusBadRequest, "Required parameter \"area\" not provided.", w)
		return
	}
	switch r.Method {
	case http.MethodGet:
		records := []Snapshot{}
		keys, err := datastore.NewQuery("Snapshot").Filter("Area =", area).Order("-TimeStamp").GetAll(ctx, &records)
		if err != nil {
			reportError(ctx, http.StatusInternalServerError, err.Error(), w)
			return
		}
		snapshots := []KeyedRecord{}
		for i, r := range records {
			snapshots = append(snapshots, KeyedRecord{Snapshot: r, Key: keys[i]})
		}
		historyTmpl.Execute(w, Listing{Area: area, Records: snapshots})
	case http.MethodPost:
		fields := []IntFieldName{id, timestamp, decibels, seated, floored}
		values, badness := extractNumbers(r, fields)
		if len(badness) != 0 {
			msg := fmt.Sprintf("Failure parsing integer parameter: %v.", badness)
			reportError(ctx, http.StatusBadRequest, msg, w)
			return
		}
		record := Snapshot{
			TimeStamp: time.Unix(int64(values[timestamp]), 0),
			Area:      area,
			People: People{
				Seated:  int8(values[seated]),
				Floored: int8(values[floored]),
			},
			Decibels: int8(values[decibels]),
		}
		key := datastore.NewKey(ctx, "Snapshot", "", int64(values[id]), nil)
		if _, err := datastore.Put(ctx, key, &record); err != nil {
			msg := fmt.Sprintf("Unable to update record in DB: %v.", err)
			reportError(ctx, http.StatusInternalServerError, msg, w)
			return
		}
		w.Header()["Location"] = append(w.Header()["Location"], fmt.Sprintf("/history?area=%s", area))
		http.Redirect(w, r, "/", http.StatusSeeOther)
	default:
		reportError(ctx, http.StatusMethodNotAllowed, fmt.Sprintf("Unsupported HTTP method %s.", r.Method), w)
	}
}

func main() {
	http.HandleFunc("/history", historyHandler)
	http.HandleFunc("/", indexHandler)
	appengine.Main()
}
