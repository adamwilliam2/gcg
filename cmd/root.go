package cmd

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strings"
	"text/template"
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
	gcgCmd.Flags().String("optionTmpl", "./templates/pkg.model.option.tmpl", "tmpl file")                  // option模板
	gcgCmd.Flags().StringVarP(&_additionalImportPkg, "additionalImportPkg", "p", "gopay", "要額外import的pkg") // 要額外加入的import file
}

func GcgRunE(_cmd *cobra.Command, _args []string) error {
	folder := _cmd.Flag("folder")
	if folder == nil || folder.Value.String() == "" {
		return errors.New("folder not defined")
	}

	optionTmpl := _cmd.Flag("optionTmpl")
	if optionTmpl == nil || optionTmpl.Value.String() == "" {
		return errors.New("option tmpl file not defined")
	}

	filesInfo, err := os.ReadDir(folder.Value.String())
	if err != nil {
		return errors.Wrap(err, "read tmpl file failed")
	}

	for _, file := range filesInfo {
		if file.IsDir() {
			continue
		}
		if strings.HasPrefix(file.Name(), "gen_") {
			continue
		}
		genFile(optionTmpl.Value.String(), folder.Value.String(), file.Name())
	}

	// format file
	{
		outputFiles, err := os.ReadDir(folder.Value.String())
		if err != nil {
			return errors.Wrap(err, "read output file folder failed")
		}
		for i := range outputFiles {
			if outputFiles[i].IsDir() {
				continue
			}
			if !strings.HasPrefix(outputFiles[i].Name(), "gen_") {
				continue
			}

			goimportsCmd := exec.Command("goimports", "-w", "-local", "gopay", outputFiles[i].Name())
			goimportsCmd.Dir = fmt.Sprintf("./%s", folder.Value.String())
			out, err := goimportsCmd.CombinedOutput()
			if err != nil {
				log.Error().Msgf("errr~ %v", err)
				log.Info().Msgf("goimports command failed %v", string(out))
				return err
			}
		}
	}

	return nil
}

func genFile(_tmplFile, _folder, _sourceFileName string) {
	filePath := _folder + "/" + _sourceFileName
	b, err := os.ReadFile(filePath)
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

	for k, value := range v.structName {
		if len(value.Fields) == 0 {
			continue
		}

		newfilePath := fmt.Sprintf("%s/gen_%s.go", _folder, toSnakeCase(k))

		fmt.Println(newfilePath)
		modelInfo := ModelInfo{
			SourceFileName: _sourceFileName,
			TmplFile:       _tmplFile,
			ModelName:      k}
		parseSourceFile(newfilePath, &modelInfo)

		{ // merge imports
			if defaultImportPkgs, exists := defaultPkgMappingByTemplates[_tmplFile]; exists {
				v.importPkgs = append(v.importPkgs, defaultImportPkgs...)
			}

			newPkgs := []ImportPkg{}
			for i := range modelInfo.oldImports {
				exists := false
				for j := range v.importPkgs {
					if modelInfo.oldImports[i].Path == v.importPkgs[j].Path {
						exists = true
						break
					}
				}
				if !exists {
					newPkgs = append(newPkgs, modelInfo.oldImports[i])
				}
			}

			v.importPkgs = append(v.importPkgs, newPkgs...)
		}

		fmt.Printf("%+v\n", modelInfo)
		for i := range value.Fields {
			value.Fields[i].NameSnake = toSnakeCase(value.Fields[i].Name)
			fmt.Println(value.Fields[i].NameSnake)
		}

		err = renderTemplate(newfilePath, _tmplFile, genTmpl{
			ModelName:           k,
			StructFields:        value.Fields,
			ModelInfo:           modelInfo,
			AdditionalImportPkg: v.importPkgs,
		})
		if err != nil {
			fmt.Printf("%v", err)
			return
		}
	}
}

var (
	commonInitialisms = []string{"API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID", "HTML", "HTTP", "HTTPS", "ID", "IP", "JSON", "LHS", "QPS", "RAM", "RHS", "RPC", "SLA", "SMS", "SMTP", "SSH", "TLS", "TTL", "UID", "UI", "UUID", "URI", "URL", "UTF8", "VM", "XML", "XSRF", "XSS"}
)

func toSnakeCase(_str string) string {

	var strReplaceWithInitialisms string = _str
	for _, initialism := range commonInitialisms {
		if strings.Contains(_str, initialism) {
			strReplaceWithInitialisms = strings.ReplaceAll(_str, initialism, string(initialism[0])+strings.ToLower(initialism[1:]))
		}
	}

	var snakeCaseStrBuilder strings.Builder
	for i, r := range strReplaceWithInitialisms {
		if unicode.IsUpper(r) {
			if i > 0 {
				snakeCaseStrBuilder.WriteByte('_')
			}
			snakeCaseStrBuilder.WriteRune(unicode.ToLower(r))
		} else {
			snakeCaseStrBuilder.WriteRune(r)
		}
	}
	return snakeCaseStrBuilder.String()
}

type visitor struct {
	file       []byte
	structName map[string]*StructFiled
	command    []Command
	importPkgs []ImportPkg
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

var defaultPkgMappingByTemplates = map[string][]ImportPkg{
	"./templates/pkg.model.option.tmpl": {
		{Alias: "", Path: "reflect"},
		{Alias: "", Path: "time"},
		{Alias: "", Path: "gorm.io/gorm"},
	},
}

func (v *visitor) shouldSkip(_genDecl *ast.GenDecl) bool {
	if _genDecl.Doc != nil {
		for _, comment := range _genDecl.Doc.List {
			if strings.Contains(comment.Text, `gcg:"gen_disable"`) || strings.Contains(comment.Text, `gcg:"-"`) {
				return true
			}
		}
	}
	return false
}

func (v *visitor) Visit(_astNode ast.Node) ast.Visitor {
	if _astNode == nil {
		return nil
	}

	switch x := _astNode.(type) {
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
						v.importPkgs = append(v.importPkgs, ImportPkg{
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

			// 依據comment 決定要不要略過這個物件
			if v.shouldSkip(x) {
				continue
			}

			v.structName[typeSpec.Name.Name] = &StructFiled{
				Fields: make([]Field, 0, len(d.Fields.List)),
			}
			for _, field := range d.Fields.List {
				// 依據欄位tag 決定要不要略過這個欄位
				if field.Tag != nil {
					if strings.Contains(field.Tag.Value, `gcg:"gen_disable"`) || strings.Contains(field.Tag.Value, `gcg:"-"`) {
						continue
					}
				}
				if len(field.Names) == 0 {
					_, ok := field.Type.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					// TODO 遍歷embeded struct
					continue
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
	SourceFileName string
	TmplFile       string

	ModelName string

	SkipWhere bool
	WhereFunc string

	SkipPreload bool
	PreloadFunc string

	SkipWhereOption   bool
	WhereOptionStruct string

	oldImports []ImportPkg
}

// Parses the Go source file and determines if the Page method should be skipped
func parseSourceFile(_filePath string, modelInfo *ModelInfo) {
	if modelInfo.oldImports == nil {
		modelInfo.oldImports = []ImportPkg{}
	}

	fset := token.NewFileSet()
	fmt.Println(_filePath)
	node, err := parser.ParseFile(fset, _filePath, nil, parser.ParseComments)
	if err != nil {
		log.Printf("err: %+v", err)
		return
	}

	src, err := os.ReadFile(_filePath)
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

			for _, spec := range x.Specs {
				if imp, ok := spec.(*ast.ImportSpec); ok {
					name := ""
					if imp.Name != nil {
						name = imp.Name.String()
					}

					modelInfo.oldImports = append(modelInfo.oldImports, ImportPkg{
						Alias: name,
						Path:  strings.Trim(imp.Path.Value, "\""),
					})
				}
			}

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

type genTmpl struct {
	ModelInfo
	ModelName           string
	StructFields        []Field
	AdditionalImportPkg []ImportPkg
}

func renderTemplate(_newfilePath string, _tmplFile string, _genTmpl genTmpl) error {
	f, err := os.Create(_newfilePath)
	if err != nil {
		return errors.New(fmt.Sprintf("os.Create new file failed: %v", err))
	}
	defer f.Close()

	t1, err := template.ParseFiles(_tmplFile)
	if err != nil {
		return errors.New(fmt.Sprintf("template.ParseFiles failed: %v", err))
	}
	t1.Option()

	err = t1.Execute(f, _genTmpl)
	if err != nil {
		return errors.New(fmt.Sprintf("template.Execute failed :%v", err))
	}
	return nil
}
