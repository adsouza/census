package main

import (
	"html/template"
	"net/http"

	"google.golang.org/appengine"
)

var (
	indexTmpl = template.Must(template.ParseFiles("index.html"))
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if r.Method == http.MethodPost {
		r.FormValue("total")
	}

	indexTmpl.Execute(w, nil)
}

func main() {
	http.HandleFunc("/", indexHandler)
	appengine.Main()
}
