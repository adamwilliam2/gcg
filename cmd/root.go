package cmd

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"html/template"
	"os"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func Execute() {
	err := gcgCmd.Execute()
	if err != nil {
		log.Error().Msgf("exec failed: %v", err)
		os.Exit(1)
	}
}

var gcgCmd = &cobra.Command{
	Use:   "gcg",
	Short: "gen code for gorm",
	Long:  `gen code for gorm, `,
	RunE:  GcgRunE,
}

var _additionalImportPkg string

func init() {
	gcgCmd.Flags().StringP("folder", "f", "", "gen source folder")                                         // 產出目錄
	gcgCmd.Flags().StringP("tmpl", "t", "./templates/pkg.model.option.tmpl", "tmpl file")                  // 模板
	gcgCmd.Flags().StringVarP(&_additionalImportPkg, "additionalImportPkg", "p", "gopay", "要額外import的pkg") // 要額外加入的import file
}

func GcgRunE(cmd *cobra.Command, args []string) error {
	folder := cmd.Flag("folder")
	if folder == nil || folder.Value.String() == "" {
		return errors.New("folder not defined")
	}

	tmplFile := cmd.Flag("tmpl")
	if tmplFile == nil || tmplFile.Value.String() == "" {
		return errors.New("tmpl file defined")
	}

	filesInfo, err := os.ReadDir(folder.Value.String())
	if err != nil {
		return errors.Wrap(err, "read tmpl file failed")
	}

	for _, file := range filesInfo {
		if file.IsDir() {
			continue
		}
		if strings.Contains(file.Name(), "gen") {
			continue
		}
		fileName := folder.Value.String() + "/" + file.Name()
		genFile(tmplFile.Value.String(), folder.Value.String(), fileName)
	}

	return nil
}

func genFile(tmplFile, folder, fileName string) {
	b, err := os.ReadFile(fileName)
	if err != nil {
		panic(err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", b, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	var v visitor
	v.file = b
	v.structName = make(map[string]*StructFiled)
	ast.Walk(&v, file)

	for k, value := range v.structName {
		fmt.Printf("%s\n%+v\n\n", k, value)
	}
	for i := range v.command {
		fmt.Printf("%+v\n", v.command[i])
	}

	t1, err := template.ParseFiles(tmplFile)
	if err != nil {
		fmt.Println("Error creating template:", err)
		return
	}

	type genTmpl struct {
		ModelName    string
		StructFields []Field
		ModelInfo
		AdditionalImportPkg []*ImportPkg
	}

	for k, value := range v.structName {
		if len(value.Fields) == 0 {
			continue
		}

		fileName := toSnakeCase(k)
		fileName = fmt.Sprintf("%s/gen_%s.go", folder, fileName)

		fmt.Println(fileName)
		modelInfo := ModelInfo{ModelName: k}
		parseSourceFile(fileName, &modelInfo)

		fmt.Printf("%+v\n", modelInfo)
		f, err := os.Create(fileName)
		if err != nil {
			fmt.Printf("create new file has failed :%v", err)
			return
		}

		defer f.Close()
		for i := range value.Fields {
			value.Fields[i].NameSnake = toSnakeCase(value.Fields[i].Name)
			fmt.Println(value.Fields[i].NameSnake)
		}
		t1.Option()
		err = t1.Execute(f, genTmpl{
			ModelName:           k,
			StructFields:        value.Fields,
			ModelInfo:           modelInfo,
			AdditionalImportPkg: v.importPkgs,
		})

		if err != nil {
			fmt.Printf("write file has failed :%v", err)
			return
		}
	}
}

var (
	commonInitialisms = []string{"API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID", "HTML", "HTTP", "HTTPS", "ID", "IP", "JSON", "LHS", "QPS", "RAM", "RHS", "RPC", "SLA", "SMS", "SMTP", "SSH", "TLS", "TTL", "UID", "UI", "UUID", "URI", "URL", "UTF8", "VM", "XML", "XSRF", "XSS"}
)

func toSnakeCase(str string) string {
	var result strings.Builder

	for _, initialism := range commonInitialisms {
		if strings.Contains(str, initialism) {
			str = strings.ReplaceAll(str, initialism, string(initialism[0])+strings.ToLower(initialism[1:]))
		}
	}

	for i, r := range str {
		if unicode.IsUpper(r) {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

type visitor struct {
	file       []byte
	structName map[string]*StructFiled
	command    []Command
	importPkgs []*ImportPkg
}

type StructFiled struct {
	Fields []Field
}

type Field struct {
	Name      string
	Type      string
	NameSnake string
}

type ImportPkg struct {
	Alias string
	Path  string
}

type Command struct {
	Pos     int
	Content string
}

func (v *visitor) shouldSkip(genDecl *ast.GenDecl) bool {
	if genDecl.Doc != nil {
		for _, comment := range genDecl.Doc.List {
			if strings.Contains(comment.Text, "gopher:gen_disable") {
				return true
			}
		}
	}
	return false
}

func (v *visitor) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return nil
	}

	switch x := n.(type) {
	case *ast.GenDecl:
		if x.Tok == token.IMPORT {
			for _, spec := range x.Specs {
				{
					importSpec, ok := spec.(*ast.ImportSpec)
					if !ok {
						continue
					}

					var aliasPkgName string
					if importSpec.Name != nil {
						aliasPkgName = importSpec.Name.Name
					}

					pkg := strings.Trim(importSpec.Path.Value, "\"")
					if strings.Contains(pkg, _additionalImportPkg) {
						v.importPkgs = append(v.importPkgs, &ImportPkg{
							Alias: aliasPkgName,
							Path:  strings.Trim(importSpec.Path.Value, "\""),
						})
					}
				}
			}
		}
		if x.Tok != token.TYPE {
			return v
		}
		for _, spec := range x.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			// Check if this type is a struct
			d, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			// Check for a specific comment
			if v.shouldSkip(x) {
				// Skip generating for this struct
				continue
			}

			v.structName[typeSpec.Name.Name] = &StructFiled{
				Fields: make([]Field, 0, len(d.Fields.List)),
			}
			for _, field := range d.Fields.List {
				if len(field.Names) == 0 {
					continue
				}
				if field.Doc != nil {
					for _, comment := range field.Doc.List {
						if strings.Contains(comment.Text, "gopher:gen_disable") {
							continue
						}
					}
				}
				v.structName[typeSpec.Name.Name].Fields = append(v.structName[typeSpec.Name.Name].Fields,
					Field{
						Name: field.Names[0].Name,
						Type: string(v.file[field.Type.Pos()-1 : field.Type.End()-1]),
					})
			}
		}
	}

	return v
}

type ModelInfo struct {
	ModelName  string
	SkipImport bool
	ImportStr  string

	SkipWhere bool
	WhereFunc string

	SkipPreload bool
	PreloadFunc string

	SkipWhereOption   bool
	WhereOptionStruct string
}

// Parses the Go source file and determines if the Page method should be skipped
func parseSourceFile(filename string, modelInfo *ModelInfo) {
	fset := token.NewFileSet()
	fmt.Println(filename)
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		log.Printf("err: %+v", err)
		return
	}

	src, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read file: %s\n", err)
		os.Exit(1)
	}

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GenDecl:
			if x.Tok != token.IMPORT {
				break
			}
			var imports []string
			for _, spec := range x.Specs {
				if imp, ok := spec.(*ast.ImportSpec); ok {
					name := ""
					if imp.Name != nil {
						name = imp.Name.String()
					}
					imports = append(imports, fmt.Sprintf("%s %s", name, imp.Path.Value))
				}
			}

			modelInfo.SkipImport = true
			modelInfo.ImportStr = fmt.Sprintf(`
import (
	%s
)`, strings.Join(imports, "\n"))
			fmt.Println(modelInfo.ImportStr)
		case *ast.FuncDecl:
			if x.Recv == nil || len(x.Recv.List) == 0 {
				return true
			}
			fn := x
			// Check if this function is a method of the specific receiver type
			receiverType, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				return true
			}

			if ident, ok := receiverType.X.(*ast.Ident); ok {
				if ident.Name == modelInfo.ModelName+"WhereOption" && fn.Name.Name == "Where" {
					modelInfo.SkipWhere = true

					funcStart := fset.Position(fn.Pos()).Offset
					funcEnd := fset.Position(fn.End()).Offset
					modelInfo.WhereFunc = string(src[funcStart:funcEnd])
				}
				if ident.Name == modelInfo.ModelName+"WhereOption" && fn.Name.Name == "Preload" {
					modelInfo.SkipPreload = true

					funcStart := fset.Position(fn.Pos()).Offset
					funcEnd := fset.Position(fn.End()).Offset
					modelInfo.PreloadFunc = string(src[funcStart:funcEnd])
				}
			}
		case *ast.TypeSpec:
			if x.Name.Name == modelInfo.ModelName+"WhereOption" {
				modelInfo.SkipWhereOption = true
				funcStart := fset.Position(x.Pos()).Offset
				funcEnd := fset.Position(x.End()).Offset
				modelInfo.WhereOptionStruct = "type" + " " + string(src[funcStart:funcEnd])
			}
		default:
			return true
		}

		return true
	})
}
