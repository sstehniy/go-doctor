package custom

import (
	"bytes"
	"go/ast"
	"go/constant"
	"go/printer"
	"go/token"
	"go/types"
	"path"
	"strings"

	"github.com/stanislavstehniy/go-doctor/internal/model"
)

func (c *analysisContext) shouldVisitDir(relDir string) bool {
	if len(c.target.PackagePatterns) == 0 {
		return true
	}
	if len(c.selectedDirs) == 0 {
		return false
	}
	_, ok := c.selectedDirs[relDir]
	return ok
}

func (c *analysisContext) diagnostic(desc Descriptor, file *sourceFile, pos token.Pos, end token.Pos, message string, symbol string) model.Diagnostic {
	position := c.fset.PositionFor(pos, false)
	endPosition := c.fset.PositionFor(end, false)
	pathname := ""
	if file != nil {
		pathname = file.relPath
	}
	return model.Diagnostic{
		Path:      pathname,
		Line:      position.Line,
		Column:    position.Column,
		EndLine:   endPosition.Line,
		EndColumn: endPosition.Column,
		Plugin:    desc.Plugin,
		Rule:      desc.Rule,
		Severity:  desc.Severity,
		Category:  desc.Category,
		Message:   message,
		Help:      desc.Help,
		Symbol:    symbol,
		Package:   file.importPath,
		Module:    file.modulePath,
	}
}

func importAliases(file *ast.File, importPath string) []string {
	var aliases []string
	for _, spec := range file.Imports {
		if unquoteImport(spec.Path.Value) != importPath {
			continue
		}
		if spec.Name == nil {
			aliases = append(aliases, path.Base(importPath))
			continue
		}
		if spec.Name.Name == "." || spec.Name.Name == "_" {
			continue
		}
		aliases = append(aliases, spec.Name.Name)
	}
	return aliases
}

func isPkgFuncCall(file *ast.File, expr ast.Expr, importPath string, names ...string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	for _, alias := range importAliases(file, importPath) {
		if ident.Name != alias {
			continue
		}
		for _, name := range names {
			if selector.Sel.Name == name {
				return true
			}
		}
	}
	return false
}

func isMethodCall(expr ast.Expr, name string) (*ast.Ident, bool) {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != name {
		return nil, false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return nil, false
	}
	return ident, true
}

func exprText(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, fset, expr)
	return buf.String()
}

func typeOf(pkg *typedPackage, expr ast.Expr) types.Type {
	if pkg == nil || pkg.pkg == nil || pkg.pkg.TypesInfo == nil {
		return nil
	}
	return pkg.pkg.TypesInfo.TypeOf(expr)
}

func objectType(pkg *typedPackage, ident *ast.Ident) types.Type {
	if pkg == nil || pkg.pkg == nil || pkg.pkg.TypesInfo == nil || ident == nil {
		return nil
	}
	if object := pkg.pkg.TypesInfo.Defs[ident]; object != nil {
		return object.Type()
	}
	if object := pkg.pkg.TypesInfo.Uses[ident]; object != nil {
		return object.Type()
	}
	return nil
}

func constantString(pkg *typedPackage, expr ast.Expr) (string, bool) {
	if pkg == nil || pkg.pkg == nil || pkg.pkg.TypesInfo == nil {
		return "", false
	}
	tv, ok := pkg.pkg.TypesInfo.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

func isNamedType(typ types.Type, packagePath string, name string) bool {
	switch typed := typ.(type) {
	case *types.Named:
		obj := typed.Obj()
		return obj != nil && obj.Name() == name && obj.Pkg() != nil && obj.Pkg().Path() == packagePath
	case *types.Pointer:
		return isNamedType(typed.Elem(), packagePath, name)
	}
	return false
}

func isStringType(typ types.Type) bool {
	if typ == nil {
		return false
	}
	basic, ok := typ.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsString != 0
}

func isErrorType(typ types.Type) bool {
	if typ == nil {
		return false
	}
	return typ.String() == "error" || isNamedType(typ, "", "error")
}

func containsMutex(typ types.Type, seen map[string]struct{}) bool {
	if typ == nil {
		return false
	}
	key := typ.String()
	if _, ok := seen[key]; ok {
		return false
	}
	seen[key] = struct{}{}

	switch typed := typ.(type) {
	case *types.Named:
		if isNamedType(typed, "sync", "Mutex") || isNamedType(typed, "sync", "RWMutex") {
			return true
		}
		return containsMutex(typed.Underlying(), seen)
	case *types.Pointer:
		return containsMutex(typed.Elem(), seen)
	case *types.Struct:
		for index := 0; index < typed.NumFields(); index++ {
			if containsMutex(typed.Field(index).Type(), seen) {
				return true
			}
		}
	}
	return false
}

func containsMutexValue(typ types.Type) bool {
	if typ == nil {
		return false
	}
	if _, ok := typ.(*types.Pointer); ok {
		return false
	}
	return containsMutex(typ, map[string]struct{}{})
}

func isContextType(typ types.Type) bool {
	return isNamedType(typ, "context", "Context")
}

func isSQLRowsType(typ types.Type) bool {
	return isNamedType(typ, "database/sql", "Rows")
}

func isSQLTxType(typ types.Type) bool {
	return isNamedType(typ, "database/sql", "Tx")
}

func isHTTPRequestType(typ types.Type) bool {
	return isNamedType(typ, "net/http", "Request")
}

func isHTTPResponseWriterType(typ types.Type) bool {
	return isNamedType(typ, "net/http", "ResponseWriter")
}

func signatureOfCall(pkg *typedPackage, call *ast.CallExpr) *types.Signature {
	typ := typeOf(pkg, call.Fun)
	switch typed := typ.(type) {
	case *types.Signature:
		return typed
	case *types.Named:
		signature, _ := typed.Underlying().(*types.Signature)
		return signature
	default:
		return nil
	}
}

func selectorIdent(expr ast.Expr) *ast.Ident {
	ident, _ := expr.(*ast.Ident)
	return ident
}

func funcDeclSignature(pkg *typedPackage, decl *ast.FuncDecl) *types.Signature {
	if pkg == nil || pkg.pkg == nil || pkg.pkg.TypesInfo == nil {
		return nil
	}
	if decl.Name == nil {
		return nil
	}
	object := pkg.pkg.TypesInfo.Defs[decl.Name]
	if object == nil {
		return nil
	}
	signature, _ := object.Type().(*types.Signature)
	return signature
}

func loopContainsCall(loop ast.Node, fn func(*ast.CallExpr) bool) bool {
	found := false
	inspectExcludingFuncLits(loop, func(node ast.Node) bool {
		if found {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if fn(call) {
			found = true
			return false
		}
		return true
	})
	return found
}

func inspectExcludingFuncLits(root ast.Node, visit func(ast.Node) bool) {
	ast.Inspect(root, func(node ast.Node) bool {
		if node != root {
			if _, ok := node.(*ast.FuncLit); ok {
				return false
			}
		}
		return visit(node)
	})
}

func isConversionToByteSlice(expr ast.Expr) (ast.Expr, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	arrayType, ok := call.Fun.(*ast.ArrayType)
	if !ok || arrayType.Len != nil {
		return nil, false
	}
	ident, ok := arrayType.Elt.(*ast.Ident)
	if !ok || ident.Name != "byte" {
		return nil, false
	}
	return call.Args[0], true
}

func isStringCompare(binary *ast.BinaryExpr) bool {
	if binary.Op != token.EQL && binary.Op != token.NEQ {
		return false
	}
	return isErrorStringExpr(binary.X) && isStringExpr(binary.Y) || isErrorStringExpr(binary.Y) && isStringExpr(binary.X)
}

func isErrorStringExpr(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Error"
}

func isStringExpr(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.BasicLit:
		return typed.Kind == token.STRING
	case *ast.BinaryExpr:
		return isStringExpr(typed.X) && isStringExpr(typed.Y)
	}
	return false
}

func pathUsesTempDir(file *ast.File, expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.BasicLit:
		return strings.Contains(typed.Value, "/tmp/")
	case *ast.BinaryExpr:
		return pathUsesTempDir(file, typed.X) || pathUsesTempDir(file, typed.Y)
	case *ast.CallExpr:
		if isPkgFuncCall(file, typed.Fun, "os", "TempDir") {
			return true
		}
		if isPkgFuncCall(file, typed.Fun, "path/filepath", "Join") {
			for _, arg := range typed.Args {
				if pathUsesTempDir(file, arg) {
					return true
				}
			}
		}
	}
	return false
}

func isMainFunction(file *sourceFile, fn *ast.FuncDecl) bool {
	return file.packageName == "main" && fn != nil && fn.Name != nil && fn.Name.Name == "main"
}
