package main

import (
	"errors"
	"flag"
	"fmt"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
)

type arrFlags []string

func (i *arrFlags) String() string {
	return ""
}

func (i *arrFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var (
	filter       = flag.String("filter", "", "筛选得结构体名.")
	tarFolder = flag.String("f", ".", "proto 输出路径")
	opn = flag.String("n", "default.proto", "proto 文件名.")
	pkgFlags     arrFlags
)

func main() {
	flag.Var(&pkgFlags, "p", `需要生成得包得路径，支持相对路径(./in)`)
	flag.Parse()

	pwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("error getting working directory: %s", err)
	}

	if len(pkgFlags) == 0 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	//ensure the path exists
	_, err = os.Stat(*tarFolder)
	if err != nil {
		log.Fatalf("error getting output file: %s", err)
	}

	pkgs, err := loadPackages(pwd, pkgFlags)
	if err != nil {
		log.Fatalf("error fetching packages: %s", err)
	}

	msgs := getMessages(pkgs, strings.ToLower(*filter))

	if err = writeOutput(msgs, *tarFolder); err != nil {
		log.Fatalf("error writing output: %s", err)
	}

	log.Printf("output file written to %s%s%s\n", pwd, string(os.PathSeparator), opn)
}

// attempt to load all packages
func loadPackages(pwd string, pkgs []string) ([]*packages.Package, error) {
	fset := token.NewFileSet()
	cfg := &packages.Config{
		Dir:  pwd,
		Mode: packages.LoadSyntax,
		Fset: fset,
	}
	packages, err := packages.Load(cfg, pkgs...)
	if err != nil {
		return nil, err
	}
	var errs = ""
	//check each loaded package for errors during loading
	for _, p := range packages {
		if len(p.Errors) > 0 {
			errs += fmt.Sprintf("error fetching package %s: ", p.String())
			for _, e := range p.Errors {
				errs += e.Error()
			}
			errs += "; "
		}
	}
	if errs != "" {
		return nil, errors.New(errs)
	}
	return packages, nil
}

type message struct {
	Name   string
	Fields []*field
}

type field struct {
	Name       string
	TypeName   string
	Order      int
	IsRepeated bool
}

func getMessages(pkgs []*packages.Package, filter string) []*message {
	var out []*message
	seen := map[string]struct{}{}
	for _, p := range pkgs {
		for _, t := range p.TypesInfo.Defs {
			if t == nil {
				continue
			}
			if !t.Exported() {
				continue
			}
			if _, ok := seen[t.Name()]; ok {
				continue
			}
			if s, ok := t.Type().Underlying().(*types.Struct); ok {
				seen[t.Name()] = struct{}{}
				if filter == "" || strings.Contains(strings.ToLower(t.Name()), filter) {
					out = appendMessage(out, t, s)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func appendMessage(out []*message, t types.Object, s *types.Struct) []*message {
	msg := &message{
		Name:   t.Name(),
		Fields: []*field{},
	}

	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if !f.Exported() {
			continue
		}
		newField := &field{
			Name:       toProtoFieldName(f.Name()),
			TypeName:   toProtoFieldTypeName(f),
			IsRepeated: isRepeated(f),
			Order:      i + 1,
		}
		msg.Fields = append(msg.Fields, newField)
	}
	out = append(out, msg)
	return out
}

func toProtoFieldTypeName(f *types.Var) string {
	switch f.Type().Underlying().(type) {
	case *types.Basic:
		name := f.Type().String()
		return normalizeType(name)
	case *types.Slice:
		name := splitNameHelper(f)
		return normalizeType(strings.TrimLeft(name, "[]"))

	case *types.Pointer, *types.Struct:
		name := splitNameHelper(f)
		return normalizeType(name)
	}
	return f.Type().String()
}

func splitNameHelper(f *types.Var) string {
	// TODO: this is ugly. Find another way of getting field type name.
	parts := strings.Split(f.Type().String(), ".")

	name := parts[len(parts)-1]

	if name[0] == '*' {
		name = name[1:]
	}
	return name
}

func normalizeType(name string) string {
	switch name {
	case "uint32", "int32":
		return "int32"
	case "int", "uint", "int64", "uint64":
		return "int64"
	case "float32":
		return "float"
	case "float64":
		return "double"
	case "time.Time", "*time.Time":
		return "int64"
	default:
		return name
	}
}

func isRepeated(f *types.Var) bool {
	_, ok := f.Type().Underlying().(*types.Slice)
	return ok
}

func toProtoFieldName(name string) string {
	if len(name) == 2 {
		return strings.ToLower(name)
	}
	data := make([]byte, 0, len(name)*2)
	j := false
	num := len(name)
	for i := 0; i < num; i++ {
		d := name[i]
		if i > 0 && d >= 'A' && d <= 'Z' && j {
			if i+1 < num {
				c := name[i+1]
				if !(c >= 'A' && c <= 'Z' && j) {
					data = append(data, '_')
				}
			}
		}
		//当前是一个大写
		if d >= 'A' && d <= 'Z' {
			//后面全部是大写
			all := true
			for n := i + 1; n < num; n++ {
				m := name[n]
				if !(m >= 'A' && m <= 'Z') {
					all = false
				}
			}
			if all {
				data = append(data, '_')
				//添加剩余并跳出
				data=append(data,name[i:]...)
				break
			}
		}
		if d != '_' {
			j = true
		}
		data = append(data, d)
	}
	return strings.ToLower(string(data[:]))
}

func writeOutput(msgs []*message, path string) error {
	msgTemplate := `syntax = "proto3";
package proto;

{{range .}}
message {{.Name}} {
{{- range .Fields}}
{{- if .IsRepeated}}
  repeated {{.TypeName}} {{.Name}} = {{.Order}};
{{- else}}
  {{.TypeName}} {{.Name}} = {{.Order}};
{{- end}}
{{- end}}
}
{{end}}
`
	tmpl, err := template.New("test").Parse(msgTemplate)
	if err != nil {
		panic(err)
	}

	f, err := os.Create(filepath.Join(path, *opn))
	if err != nil {
		return fmt.Errorf("unable to create file %s : %s", *opn, err)
	}
	defer f.Close()

	return tmpl.Execute(f, msgs)
}
