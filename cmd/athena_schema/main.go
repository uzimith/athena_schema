package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"
	"unicode"
)

var (
	typeNameList     = flag.String("type", "", "comma-separated list of type names; must be set")
	tableNameList    = flag.String("table", "", "comma-separated list of table names; If table name is empty, default name is used.")
	folderNameList   = flag.String("folder", "", "comma-separated list of folder names; If folder name is empty, table name is used.")
	folderNamePrefix = flag.String("prefix", "", "folder name prefix")
	folderNameSuffix = flag.String("suffix", "", "folder name suffix")
	output           = flag.String("output", "", "output file name; default srcdir/<type>_athena.sql")
	templatePath     = flag.String("template", ".", "template file: {templatePath}/template.tpl")
)

func Usage() {
	io.WriteString(os.Stderr, usageText)
	flag.PrintDefaults()
}

const usageText = `athena-schema
Example:
	athena-schema -type=User [directory]
	athena-schema -type=User files... # Must be a single package

Flags:
`

func main() {
	log.SetFlags(0)
	log.SetPrefix("athena_schema: ")
	flag.Usage = Usage
	flag.Parse()
	if len(*typeNameList) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	typeNames := strings.Split(*typeNameList, ",")
	tableNames := make([]string, len(typeNames))
	if tableNameList != nil {
		list := strings.Split(*tableNameList, ",")
		copy(tableNames, list)
	}
	folderNames := make([]string, len(typeNames))
	if folderNameList != nil {
		list := strings.Split(*folderNameList, ",")
		copy(folderNames, list)
	}

	folderNamePrefixStr := ""
	if folderNamePrefix != nil {
		folderNamePrefixStr = *folderNamePrefix
	}

	folderNameSuffixStr := ""
	if folderNameSuffix != nil {
		folderNameSuffixStr = *folderNameSuffix
	}

	// We accept either one directory or a list of files. Which do we have?
	args := flag.Args()
	if len(args) == 0 {
		// Default: process whole package in current directory.
		args = []string{"."}
	}

	// Parse the package once.
	var dir string
	g := Generator{
		tables: make([]Table, 0, len(typeNames)),
		info: &types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
			Defs:  make(map[*ast.Ident]types.Object),
			Uses:  make(map[*ast.Ident]types.Object),
		},
	}

	if len(args) == 1 && isDirectory(args[0]) {
		dir = args[0]
		g.parsePackageDir(args[0])
	} else {
		dir = filepath.Dir(args[0])
		g.parsePackageFiles(args)
	}

	for i, typeName := range typeNames {
		tableName := tableNames[i]
		folderName := folderNames[i]
		g.generate(&State{
			typeName:   typeName,
			tableName:  tableName,
			folderName: folderName,
		})
	}

	// Format the output.
	src := g.format(*templatePath, folderNamePrefixStr, folderNameSuffixStr)

	// Write to file.
	outputName := *output
	if outputName == "" {
		baseName := fmt.Sprintf("%s_athena.sql", CamelToSnake(typeNames[0]))
		outputName = filepath.Join(dir, baseName)
	}

	err := ioutil.WriteFile(outputName, src, 0644)
	if err != nil {
		log.Fatalf("writing output: %s", err)
	}
}

func isDirectory(name string) bool {
	info, err := os.Stat(name)
	if err != nil {
		log.Fatal(err)
	}
	return info.IsDir()
}

type Generator struct {
	// for tmpl
	tables []Table

	// go/types generated values
	info        *types.Info
	pkg         *types.Package
	packageName string
}

type State struct {
	typeName   string
	tableName  string
	folderName string
}

type Table struct {
	TableName  string
	FolderName string
	Columns    []Column
}

type Column struct {
	Name string
	Type string
}

type Tmpl struct {
	CmdLog           string
	PackageName      string
	FolderNamePrefix string
	FolderNameSuffix string
	Tables           []Table
}

func (g *Generator) parsePackageDir(directory string) {
	pkg, err := build.Default.ImportDir(directory, 0)
	if err != nil {
		log.Fatalf("cannot process directory %s: %s", directory, err)
	}
	var names []string
	names = append(names, pkg.GoFiles...)
	names = append(names, pkg.CgoFiles...)
	names = append(names, pkg.SFiles...)
	names = prefixDirectory(directory, names)
	g.parsePackage(directory, names, nil)
}

func (g *Generator) parsePackageFiles(names []string) {
	g.parsePackage(".", names, nil)
}

func prefixDirectory(directory string, names []string) []string {
	if directory == "." {
		return names
	}
	ret := make([]string, len(names))
	for i, name := range names {
		ret[i] = filepath.Join(directory, name)
	}
	return ret
}

func (g *Generator) parsePackage(directory string, names []string, text interface{}) {
	var files []*ast.File
	fs := token.NewFileSet()
	for _, name := range names {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		parsedFile, err := parser.ParseFile(fs, name, text, parser.ParseComments)
		if err != nil {
			log.Fatalf("parsing package: %s: %s", name, err)
		}
		files = append(files, parsedFile)
	}
	if len(files) == 0 {
		log.Fatalf("%s: no buildable Go files", directory)
	}

	g.packageName = files[0].Name.Name
	config := types.Config{
		IgnoreFuncBodies:         false,
		DisableUnusedImportCheck: true,
		Importer:                 importer.For("source", nil),
		FakeImportC:              true,
	}
	typesPkg, err := config.Check(directory, fs, files, g.info)
	if err != nil {
		if typesErr, ok := err.(types.Error); ok {
			if typesErr.Soft {
				log.Printf("checking package soft failed: %s", typesErr.Error())
			} else {
				log.Fatalf("checking package: %s", typesErr.Error())
			}
		} else {
			log.Fatalf("checking package: %s", err.Error())
		}
	}
	g.pkg = typesPkg
}

func (g *Generator) generate(state *State) {
	structType, ok := state.searchStruct(g.pkg)

	if !ok {
		log.Fatalf("Not found specifed name struct: %s", state.typeName)
	}

	columns := genCoulmns(structType)

	if state.tableName == "" {
		state.tableName = CamelToSnake(state.typeName)
	}

	if state.folderName == "" {
		state.folderName = state.tableName
	}

	table := Table{
		TableName:  state.tableName,
		FolderName: state.folderName,
		Columns:    columns,
	}
	g.tables = append(g.tables, table)
}

func (state *State) searchStruct(pkg *types.Package) (*types.Struct, bool) {
	if object, ok := pkg.Scope().Lookup(state.typeName).(*types.TypeName); ok {
		if structType, ok := object.Type().Underlying().(*types.Struct); ok {
			return structType, true
		}
	}

	for _, childPkg := range pkg.Imports() {
		if structType, ok := state.searchStruct(childPkg); ok {
			return structType, true
		}
	}

	return nil, false
}

func genCoulmns(fields *types.Struct) []Column {
	columns := make([]Column, 0, fields.NumFields())
	for i := 0; i < fields.NumFields(); i++ {
		field := fields.Field(i)
		tags := reflect.StructTag(fields.Tag(i))
		name := CamelToSnake(field.Name())
		sqlType := ""

		jsonTag, ok := tags.Lookup("json")
		if ok {
			jsonTags := strings.Split(jsonTag, ",")
			name = jsonTags[0]
		}

		athenaType, ok := tags.Lookup("athena")
		if ok {
			sqlType = athenaType
		}

		if name == "-" || sqlType == "-" {
			continue
		}

		if sqlType == "" {
			sqlTypeByFieldType, ok := genSqlType(field.Type())
			if ok {
				sqlType = sqlTypeByFieldType
			} else {
				log.Fatalf("no support field type: %s", field.Name())
			}
		}

		column := Column{
			Name: name,
			Type: sqlType,
		}
		columns = append(columns, column)
	}
	return columns
}

func genSqlType(fieldType types.Type) (string, bool) {
	sqlType, ok := SQLTypeMap[fieldType.String()]
	if ok {
		return sqlType, true
	}

	switch typeKind := fieldType.(type) {
	case *types.Slice:
		typeStr, ok := genSqlType(typeKind.Elem())
		if !ok {
			return "", false
		}
		return fmt.Sprintf("array<%s>", typeStr), true
	case *types.Array:
		typeStr, ok := genSqlType(typeKind.Elem())
		if !ok {
			return "", false
		}
		return fmt.Sprintf("array<%s>", typeStr), true
	case *types.Struct:
		columns := genCoulmns(typeKind)
		columnStrs := make([]string, 0, len(columns))
		for _, column := range columns {
			columnStrs = append(columnStrs, fmt.Sprintf("%s: %s", column.Name, column.Type))
		}
		return fmt.Sprintf("struct<%s>", strings.Join(columnStrs, ", ")), true
	case *types.Map:
		key, ok := genSqlType(typeKind.Key())
		if !ok {
			return "", false
		}
		value, ok := genSqlType(typeKind.Elem())
		if !ok {
			return "", false
		}
		return fmt.Sprintf("map<%s, %s>", key, value), true
	case *types.Pointer:
		typeStr, ok := genSqlType(typeKind.Elem())
		if !ok {
			return "", false
		}
		return typeStr, true
	case *types.Named:
		return genSqlType(fieldType.Underlying())
	default:
		return "", false
	}
}

var tmplFuncs = template.FuncMap{
	"last": func(x int, a interface{}) bool {
		return x == reflect.ValueOf(a).Len()-1
	},
}

func (g *Generator) format(templatePath string, folderNamePrefix string, folderNameSuffix string) []byte {
	templateFile := fmt.Sprintf("%s/template.tpl", templatePath)
	tname := filepath.Base(templateFile)
	tmpl, err := template.New(tname).Funcs(tmplFuncs).ParseFiles(templateFile)

	if err != nil {
		log.Fatalf("Template %v parse error: %s", templatePath, err.Error())
	}

	newbytes := bytes.NewBufferString("")
	t := &Tmpl{
		CmdLog:           fmt.Sprintf("athena_schema %s", strings.Join(os.Args[1:], " ")),
		PackageName:      g.packageName,
		FolderNamePrefix: folderNamePrefix,
		FolderNameSuffix: folderNameSuffix,
		Tables:           g.tables,
	}

	err = tmpl.Execute(newbytes, t)

	if err != nil {
		log.Fatalf("%v", err)
	}

	tplcontent, err := ioutil.ReadAll(newbytes)

	if err != nil {
		log.Fatalf("%v", err)
	}

	return tplcontent
}

var SQLTypeMap = map[string]string{
	"bool":      "boolean",
	"string":    "string",
	"int":       "int",
	"int8":      "int",
	"int16":     "int",
	"int32":     "int",
	"int64":     "int",
	"uint8":     "int",
	"uint16":    "int",
	"uint32":    "int",
	"uint64":    "int",
	"float32":   "float",
	"float64":   "double",
	"[]byte":    "string",
	"time.Time": "timestamp",
}

func CamelToSnake(s string) string {
	var result string
	var words []string
	var lastPos int
	rs := []rune(s)

	for i := 0; i < len(rs); i++ {
		if i > 0 && unicode.IsUpper(rs[i]) {
			if initialism := startsWithInitialism(s[lastPos:]); initialism != "" {
				words = append(words, initialism)

				i += len(initialism) - 1
				lastPos = i
				continue
			}

			words = append(words, s[lastPos:i])
			lastPos = i
		}
	}

	// append the last word
	if s[lastPos:] != "" {
		words = append(words, s[lastPos:])
	}

	for k, word := range words {
		if k > 0 {
			result += "_"
		}

		result += strings.ToLower(word)
	}

	return result
}

// startsWithInitialism returns the initialism if the given string begins with it
func startsWithInitialism(s string) string {
	var initialism string
	// the longest initialism is 5 char, the shortest 2
	for i := 1; i <= 5; i++ {
		if len(s) > i-1 && commonInitialisms[s[:i]] {
			initialism = s[:i]
		}
	}
	return initialism
}

// commonInitialisms, taken from
// https://github.com/golang/lint/blob/206c0f020eba0f7fbcfbc467a5eb808037df2ed6/lint.go#L731
var commonInitialisms = map[string]bool{
	"ACL":   true,
	"API":   true,
	"ASCII": true,
	"CPU":   true,
	"CSS":   true,
	"DNS":   true,
	"EOF":   true,
	"ETA":   true,
	"GPU":   true,
	"GUID":  true,
	"HTML":  true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"IP":    true,
	"JSON":  true,
	"LHS":   true,
	"OS":    true,
	"QPS":   true,
	"RAM":   true,
	"RHS":   true,
	"RPC":   true,
	"SLA":   true,
	"SMTP":  true,
	"SQL":   true,
	"SSH":   true,
	"TCP":   true,
	"TLS":   true,
	"TTL":   true,
	"UDP":   true,
	"UI":    true,
	"UID":   true,
	"UUID":  true,
	"URI":   true,
	"URL":   true,
	"UTF8":  true,
	"VM":    true,
	"XML":   true,
	"XMPP":  true,
	"XSRF":  true,
	"XSS":   true,
	"OAuth": true,
}
