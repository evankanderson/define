package main

import (
	"fmt"

	"github.com/ogier/pflag"

	"github.com/Rican7/define/internal/config"
	"github.com/Rican7/define/internal/version"
	"github.com/Rican7/define/registry"
	"github.com/Rican7/define/source"
	"github.com/Rican7/define/source/oxford"

	//	_ "github.com/Rican7/define/source/glosbe"
	_ "github.com/Rican7/define/source/webster"

	"html/template"
	"net/http"
	"os"
)

var (
	src     source.Source
	httpTop = template.Must(template.New("body").Parse(`
<html>
	<head><title>Define</title></head>
	<body>
	<h1>Define!</h1>
	<form method="get" action="/define">
		<label for="q">Define</label>
		<input type="text" id="q" name="q" onchange="this.form.submit()">
	</form>
	{{- if .Word}}
	<hr>
	<h2>{{.Word}}</h2>
	<ol>
	{{range .Definition}}
		<li><p><em>{{.}}</em>
	{{end}}
	</ol>
	{{end}}
	</body>
</html>`))
)

func init() {
	flags := pflag.NewFlagSet(version.AppName, pflag.ContinueOnError)
	flags.Usage = func() {
		fmt.Printf("Usage: %s\n", version.AppName)
		fmt.Printf("Set $PORT for the to port to listen on.\n")
		os.Exit(2)
	}
	providers := registry.ConfigureProviders(flags)

	if len(providers) < 1 {
		fmt.Printf("Must set at least one provider!\n")
		os.Exit(1)
	}
	providerList := make([]registry.Configuration, 0, len(providers))
	for _, p := range providers {
		providerList = append(providerList, p)
	}

	conf, err := config.NewFromRuntime(flags, providers, "/dev/null", config.Configuration{
		PreferredSource: oxford.JSONKey,
	})
	if err != nil {
		fmt.Printf("Failed to configure: %w\n", err)
		os.Exit(1)
	}
	registry.Finalize(providerList...)

	src, err = registry.ProvidePreferred(conf.PreferredSource, providerList)
	if err != nil {
		fmt.Printf("Failed fetch preferred provider: %w\n", err)
		os.Exit(1)
	}
}

func handle(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(200)
	err := httpTop.Execute(w, nil)
	if err != nil {
		fmt.Printf("Error processing %q: %+v\n", req.URL.Path, err)
	}
}

func defineHttp(w http.ResponseWriter, req *http.Request) {
	result := struct {
		Word       string
		Definition []string
	}{
		Word: req.URL.Query().Get("q"),
	}
	if result.Word != "" {
		if src == nil {
			fmt.Printf("`src` is nil, somehow...\n")
			return
		}
		def, err := src.Define(result.Word)
		if err != nil {
			fmt.Printf("Error defining %q: %+v\n", result.Word, err)
			result.Word = err.Error()
			if responseErr, ok := err.(*source.InvalidResponseError); ok {
				result.Definition = []string{
					fmt.Sprintf("+%v", responseErr.HttpResponse),
				}
				fmt.Printf("Detail: %+v\n", responseErr.HttpResponse)
			}
		} else {
			for _, entry := range def.Entries() {
				for _, sense := range entry.Senses() {
					for _, d := range sense.Definitions() {
						result.Definition = append(result.Definition, d)
					}
				}
			}
		}
	}

	w.WriteHeader(200)

	httpTop.Execute(w, result)
}

func main() {
	listenAddr := ":" + os.Getenv("PORT")
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	http.HandleFunc("/", handle)
	http.HandleFunc("/define", defineHttp)

	fmt.Printf("Listening on %s\n", listenAddr)
	http.ListenAndServe(listenAddr, nil)
}
