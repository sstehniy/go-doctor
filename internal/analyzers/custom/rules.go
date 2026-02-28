package custom

import (
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"strings"

	"github.com/stanislavstehniy/go-doctor/internal/model"
)

const (
	oversizedPackageDeclThreshold = 150
	oversizedPackageFileThreshold = 20
	cohesiveFileLineAllowance     = 500
	godFileDeclThreshold          = 120
	godFileLineThreshold          = 2000
)

var registry = []rule{
	defineRule("error/ignored-return", "correctness", false, "warning", "Ignored error return", "Capture and handle returned errors instead of discarding them.", runIgnoredReturn),
	defineRule("error/string-compare", "correctness", true, "warning", "Error string comparison", "Compare sentinel errors with errors.Is or concrete types with errors.As.", runErrorStringCompare),
	defineRule("error/fmt-error-without-wrap", "correctness", true, "warning", "fmt.Errorf without %w", "Use %w when propagating an existing error with fmt.Errorf.", runFmtErrorWithoutWrap),
	defineRule("context/missing-first-arg", "context", false, "warning", "Context missing from function signature", "Accept context.Context as the first parameter when the function calls context-aware APIs.", runContextMissingFirstArg),
	defineRule("context/background-in-request-path", "context", true, "warning", "Background context in request path", "Propagate request context instead of starting from context.Background or context.TODO.", runContextBackgroundInRequestPath),
	defineRule("context/not-propagated", "context", true, "warning", "Context not propagated", "Pass the incoming context through downstream calls instead of replacing it.", runContextNotPropagated),
	defineRule("context/with-timeout-not-canceled", "context", true, "warning", "Context cancel function not used", "Always call the cancel function returned by context.WithCancel/WithTimeout/WithDeadline.", runContextWithTimeoutNotCanceled),
	defineRule("concurrency/go-routine-leak-risk", "concurrency", true, "error", "Goroutine leak risk", "Tie long-lived goroutines to cancellation or an explicit shutdown signal.", runGoroutineLeakRisk),
	defineRule("concurrency/ticker-not-stopped", "concurrency", true, "warning", "Ticker not stopped", "Stop tickers when the owning scope exits to avoid leaking timers.", runTickerNotStopped),
	defineRule("concurrency/waitgroup-misuse", "concurrency", true, "error", "WaitGroup misuse", "Call WaitGroup.Add before launching the goroutine.", runWaitGroupMisuse),
	defineRule("concurrency/mutex-copy", "concurrency", true, "warning", "Mutex copied by value", "Use pointer receivers and avoid copying structs that embed sync.Mutex or sync.RWMutex.", runMutexCopy),
	defineRule("perf/defer-in-hot-loop", "performance", true, "warning", "Defer in loop", "Move deferred cleanup out of hot loops or close values explicitly inside the loop.", runDeferInHotLoop),
	defineRule("perf/fmt-sprint-simple-concat", "performance", false, "warning", "fmt sprint for simple concat", "Use string concatenation when all operands are already strings.", runFmtSprintSimpleConcat),
	defineRule("perf/bytes-buffer-copy", "performance", true, "warning", "bytes.Buffer extra copy", "Prefer bytes.NewBufferString or WriteString when starting from a string.", runBytesBufferCopy),
	defineRule("perf/json-unmarshal-twice", "performance", true, "warning", "Repeated json.Unmarshal", "Decode once and reuse the parsed value instead of unmarshaling the same payload twice.", runJSONUnmarshalTwice),
	defineRule("net/http-request-without-context", "net/http", false, "warning", "HTTP request without context", "Use http.NewRequestWithContext so request cancellation and deadlines propagate.", runHTTPRequestWithoutContext),
	defineRule("net/http-default-client", "net/http", true, "warning", "Default HTTP client usage", "Create an explicit http.Client with timeouts instead of using the package default.", runHTTPDefaultClient),
	defineRule("net/http-server-no-timeouts", "net/http", true, "error", "HTTP server without timeouts", "Configure ReadHeaderTimeout and related timeouts on public servers.", runHTTPServerNoTimeouts),
	defineRule("time/tick-leak", "resource", true, "warning", "time.Tick leak", "Use time.NewTicker and stop it when the owner exits.", runTimeTickLeak),
	defineRule("time/after-in-loop", "resource", true, "warning", "time.After in loop", "Reuse a timer in loops instead of allocating a new timer on every iteration.", runTimeAfterInLoop),
	defineRule("db/rows-not-closed", "resource", true, "error", "Rows not closed", "Close *sql.Rows values, usually with defer rows.Close().", runRowsNotClosed),
	defineRule("db/rows-err-not-checked", "resource", true, "error", "Rows error not checked", "Check rows.Err after iteration completes.", runRowsErrNotChecked),
	defineRule("db/tx-no-deferred-rollback", "resource", true, "error", "Transaction missing deferred rollback", "Defer tx.Rollback immediately after Begin/BeginTx succeeds.", runTxNoDeferredRollback),
	defineRule("io/readall-unbounded", "resource", false, "warning", "Unbounded ReadAll", "Wrap body reads with io.LimitReader or otherwise bound input size before reading all bytes.", runReadAllUnbounded),
	defineRule("arch/cross-layer-import", "architecture", true, "warning", "Cross-layer import", "Keep imports aligned with the configured architecture layers.", runCrossLayerImport),
	defineRule("arch/forbidden-package-cycles", "architecture", true, "error", "Package import cycle", "Break repository package cycles so packages can be reasoned about independently.", runForbiddenPackageCycles),
	defineRule("arch/oversized-package", "architecture", true, "warning", "Oversized package", "Split packages that accumulate too many files or declarations.", runOversizedPackage),
	defineRule("arch/god-file", "architecture", true, "warning", "God file", "Split very large source files into focused units.", runGodFile),
	defineRule("test/missing-table-driven", "testing", false, "warning", "Missing table-driven test", "Use table-driven tests when a single test repeats the same call pattern many times.", runMissingTableDriven),
	defineRule("test/no-assertions", "testing", false, "warning", "Test without assertions", "Ensure tests fail when behavior changes by asserting or checking outcomes.", runNoAssertions),
	defineRule("test/sleep-in-test", "testing", true, "warning", "Sleep in test", "Use synchronization or polling helpers instead of fixed sleeps in tests.", runSleepInTest),
	defineRule("test/http-handler-no-test", "testing", true, "warning", "HTTP handler without test", "Add a focused httptest-based test for each handler.", runHTTPHandlerNoTest),
	defineRule("sec/math-rand-for-secret", "security", true, "error", "math/rand used for secrets", "Use crypto/rand for tokens, passwords, keys, and other secrets.", runMathRandForSecret),
	defineRule("sec/insecure-temp-file", "security", true, "warning", "Insecure temp file path", "Use os.CreateTemp instead of building predictable temp-file paths yourself.", runInsecureTempFile),
	defineRule("sec/exec-user-input", "security", true, "error", "Exec with user input", "Avoid passing untrusted input directly to os/exec.", runExecUserInput),
	defineRule("api/error-string-branching", "api-surface", true, "warning", "Branching on error strings", "Branch on concrete errors with errors.Is or errors.As instead of string matching.", runErrorStringBranching),
	defineRule("api/exported-mutable-global", "api-surface", true, "warning", "Exported mutable global", "Prefer constants, accessors, or immutable sentinels over exported package variables.", runExportedMutableGlobal),
	defineRule("api/init-side-effects", "api-surface", true, "warning", "init side effects", "Keep init functions free of I/O, goroutines, and global process side effects.", runInitSideEffects),
	defineRule("lib/os-exit-in-non-main", "library-safety", true, "error", "os.Exit outside main", "Return errors from libraries instead of terminating the process.", runOSExitInNonMain),
	defineRule("lib/flag-parse-in-non-main", "library-safety", true, "warning", "flag.Parse outside main", "Leave flag parsing to main so libraries remain embeddable and testable.", runFlagParseInNonMain),
}

func defineRule(name string, category string, defaultOn bool, severity string, description string, help string, fn func(*analysisContext, Descriptor) []model.Diagnostic) rule {
	desc := Descriptor{
		Plugin:      "custom",
		Rule:        name,
		Category:    category,
		DefaultOn:   defaultOn,
		Severity:    severity,
		Description: description,
		Help:        help,
	}
	return rule{
		desc: desc,
		run: func(pass *analysisContext) []model.Diagnostic {
			return fn(pass, desc)
		},
	}
}

func runIgnoredReturn(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, pkg := range pass.typedPackages {
		for _, file := range pkg.files {
			ast.Inspect(file.file, func(node ast.Node) bool {
				stmt, ok := node.(*ast.ExprStmt)
				if !ok {
					return true
				}
				call, ok := stmt.X.(*ast.CallExpr)
				if !ok {
					return true
				}
				typ := typeOf(pkg, call)
				switch typed := typ.(type) {
				case *types.Tuple:
					if typed.Len() == 0 {
						return true
					}
					if isErrorType(typed.At(typed.Len() - 1).Type()) {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, call.Pos(), call.End(), "ignored return value that includes an error", ""))
					}
				default:
					if isErrorType(typ) {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, call.Pos(), call.End(), "ignored error return value", ""))
					}
				}
				return true
			})
		}
	}
	return diagnostics
}

func runErrorStringCompare(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, pkg := range pass.typedPackages {
		for _, file := range pkg.files {
			ast.Inspect(file.file, func(node ast.Node) bool {
				binary, ok := node.(*ast.BinaryExpr)
				if !ok || !isStringCompare(binary) {
					return true
				}
				diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, binary.Pos(), binary.End(), "error compared to a string value", ""))
				return true
			})
		}
	}
	return diagnostics
}

func runFmtErrorWithoutWrap(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, pkg := range pass.typedPackages {
		for _, file := range pkg.files {
			ast.Inspect(file.file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || !isPkgFuncCall(file.file, call.Fun, "fmt", "Errorf") || len(call.Args) < 2 {
					return true
				}
				format, ok := constantString(pkg, call.Args[0])
				if !ok || strings.Contains(format, "%w") {
					return true
				}
				for _, arg := range call.Args[1:] {
					if isErrorType(typeOf(pkg, arg)) {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, call.Pos(), call.End(), "fmt.Errorf uses an error argument without %w wrapping", ""))
						return false
					}
				}
				return true
			})
		}
	}
	return diagnostics
}

func runContextMissingFirstArg(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, pkg := range pass.typedPackages {
		for _, file := range pkg.files {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				signature := funcDeclSignature(pkg, fn)
				if signature == nil {
					continue
				}
				hasHTTPRequest := false
				ctxIndex := -1
				for index := 0; index < signature.Params().Len(); index++ {
					if isHTTPRequestType(signature.Params().At(index).Type()) {
						hasHTTPRequest = true
					}
					if isContextType(signature.Params().At(index).Type()) {
						ctxIndex = index
						break
					}
				}
				if hasHTTPRequest {
					continue
				}
				if ctxIndex > 0 {
					diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, fn.Pos(), fn.Name.End(), "context.Context should be the first parameter", fn.Name.Name))
					continue
				}
				if ctxIndex == 0 {
					continue
				}
				reported := false
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					sig := signatureOfCall(pkg, call)
					if sig == nil || sig.Params().Len() == 0 || !isContextType(sig.Params().At(0).Type()) {
						return true
					}
					diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, fn.Pos(), fn.Name.End(), "function calls context-aware APIs but does not accept context.Context", fn.Name.Name))
					reported = true
					return false
				})
				if reported {
					continue
				}
			}
		}
	}
	return diagnostics
}

func runContextBackgroundInRequestPath(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, pkg := range pass.typedPackages {
		for _, file := range pkg.files {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				signature := funcDeclSignature(pkg, fn)
				if signature == nil {
					continue
				}
				isRequestPath := false
				for index := 0; index < signature.Params().Len(); index++ {
					if isHTTPRequestType(signature.Params().At(index).Type()) || isHTTPResponseWriterType(signature.Params().At(index).Type()) {
						isRequestPath = true
						break
					}
				}
				if !isRequestPath {
					continue
				}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					if isPkgFuncCall(file.file, call.Fun, "context", "Background", "TODO") {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, call.Pos(), call.End(), "request-path code starts a new background context", fn.Name.Name))
					}
					return true
				})
			}
		}
	}
	return diagnostics
}

func runContextNotPropagated(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, pkg := range pass.typedPackages {
		for _, file := range pkg.files {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				signature := funcDeclSignature(pkg, fn)
				if signature == nil || signature.Params().Len() == 0 || !isContextType(signature.Params().At(0).Type()) {
					continue
				}
				allowed := map[string]struct{}{signature.Params().At(0).Name(): {}}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if ok && isPkgFuncCall(file.file, call.Fun, "context", "Background", "TODO") {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, call.Pos(), call.End(), "incoming context is replaced with a new root context", fn.Name.Name))
						return true
					}
					assign, ok := node.(*ast.AssignStmt)
					if ok && len(assign.Lhs) > 0 && len(assign.Rhs) == 1 {
						call, ok := assign.Rhs[0].(*ast.CallExpr)
						if ok && isPkgFuncCall(file.file, call.Fun, "context", "WithCancel", "WithTimeout", "WithDeadline") && len(call.Args) > 0 {
							if ident := selectorIdent(call.Args[0]); ident != nil {
								if _, ok := allowed[ident.Name]; ok {
									if lhs, ok := assign.Lhs[0].(*ast.Ident); ok {
										allowed[lhs.Name] = struct{}{}
									}
								}
							}
						}
					}
					call, ok = node.(*ast.CallExpr)
					if !ok {
						return true
					}
					sig := signatureOfCall(pkg, call)
					if sig == nil || sig.Params().Len() == 0 || !isContextType(sig.Params().At(0).Type()) || len(call.Args) == 0 {
						return true
					}
					if ident := selectorIdent(call.Args[0]); ident != nil {
						if _, ok := allowed[ident.Name]; ok {
							return true
						}
					}
					if isPkgFuncCall(file.file, call.Args[0], "context", "Background", "TODO") {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, call.Pos(), call.End(), "incoming context is not propagated to downstream call", fn.Name.Name))
					}
					return true
				})
			}
		}
	}
	return diagnostics
}

func runContextWithTimeoutNotCanceled(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.nonTest {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				cancelDefs := map[string]token.Pos{}
				cancelCalls := map[string]bool{}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					switch typed := node.(type) {
					case *ast.AssignStmt:
						if len(typed.Lhs) < 2 || len(typed.Rhs) != 1 {
							return true
						}
						call, ok := typed.Rhs[0].(*ast.CallExpr)
						if !ok || !isPkgFuncCall(file.file, call.Fun, "context", "WithCancel", "WithTimeout", "WithDeadline") {
							return true
						}
						if ident, ok := typed.Lhs[len(typed.Lhs)-1].(*ast.Ident); ok {
							cancelDefs[ident.Name] = call.Pos()
						}
					case *ast.CallExpr:
						if ident, ok := typed.Fun.(*ast.Ident); ok {
							if _, exists := cancelDefs[ident.Name]; exists {
								cancelCalls[ident.Name] = true
							}
						}
					}
					return true
				})
				for name, pos := range cancelDefs {
					if cancelCalls[name] {
						continue
					}
					diagnostics = append(diagnostics, pass.diagnostic(desc, file, pos, pos, "cancel function returned by context.With* is never called", fn.Name.Name))
				}
			}
		}
	}
	return diagnostics
}

func runGoroutineLeakRisk(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.nonTest {
			ast.Inspect(file.file, func(node ast.Node) bool {
				goStmt, ok := node.(*ast.GoStmt)
				if !ok {
					return true
				}
				call, ok := goStmt.Call.Fun.(*ast.FuncLit)
				if !ok || call.Body == nil {
					return true
				}
				hasInfiniteLoop := false
				ast.Inspect(call.Body, func(child ast.Node) bool {
					loop, ok := child.(*ast.ForStmt)
					if ok && loop.Cond == nil && loop.Post == nil {
						hasInfiniteLoop = true
						return false
					}
					return true
				})
				if !hasInfiniteLoop {
					return true
				}
				hasCancellation := false
				ast.Inspect(call.Body, func(child ast.Node) bool {
					selector, ok := child.(*ast.SelectorExpr)
					if ok && selector.Sel.Name == "Done" {
						hasCancellation = true
						return false
					}
					return true
				})
				if !hasCancellation {
					diagnostics = append(diagnostics, pass.diagnostic(desc, file, goStmt.Pos(), goStmt.End(), "goroutine starts an unbounded loop without cancellation", ""))
				}
				return true
			})
		}
	}
	return diagnostics
}

func runTickerNotStopped(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	return runMethodResourceRule(pass, desc, "time", "NewTicker", "Stop", "ticker created with time.NewTicker is never stopped")
}

func runWaitGroupMisuse(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.nonTest {
			ast.Inspect(file.file, func(node ast.Node) bool {
				goStmt, ok := node.(*ast.GoStmt)
				if !ok {
					return true
				}
				lit, ok := goStmt.Call.Fun.(*ast.FuncLit)
				if !ok || lit.Body == nil {
					return true
				}
				ast.Inspect(lit.Body, func(child ast.Node) bool {
					call, ok := child.(*ast.CallExpr)
					if !ok {
						return true
					}
					ident, ok := isMethodCall(call.Fun, "Add")
					if !ok {
						return true
					}
					diagnostics = append(diagnostics, pass.diagnostic(desc, file, call.Pos(), call.End(), "WaitGroup.Add called from inside a goroutine", ident.Name))
					return true
				})
				return true
			})
		}
	}
	return diagnostics
}

func runMutexCopy(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, pkg := range pass.typedPackages {
		for _, file := range pkg.files {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
					continue
				}
				receiverType := typeOf(pkg, fn.Recv.List[0].Type)
				if _, ok := receiverType.(*types.Pointer); ok {
					continue
				}
				if containsMutex(receiverType, map[string]struct{}{}) {
					diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, fn.Pos(), fn.Name.End(), "method copies a receiver that contains a mutex", fn.Name.Name))
				}
			}
			ast.Inspect(file.file, func(node ast.Node) bool {
				assign, ok := node.(*ast.AssignStmt)
				if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
					return true
				}
				if containsMutexValue(typeOf(pkg, assign.Rhs[0])) {
					diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, assign.Pos(), assign.End(), "assignment copies a value that contains a mutex", ""))
				}
				return true
			})
		}
	}
	return diagnostics
}

func runDeferInHotLoop(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.nonTest {
			ast.Inspect(file.file, func(node ast.Node) bool {
				switch loop := node.(type) {
				case *ast.ForStmt:
					inspectExcludingFuncLits(loop.Body, func(child ast.Node) bool {
						deferStmt, ok := child.(*ast.DeferStmt)
						if !ok {
							return true
						}
						diagnostics = append(diagnostics, pass.diagnostic(desc, file, deferStmt.Pos(), deferStmt.End(), "defer executed inside a loop", ""))
						return true
					})
				case *ast.RangeStmt:
					inspectExcludingFuncLits(loop.Body, func(child ast.Node) bool {
						deferStmt, ok := child.(*ast.DeferStmt)
						if !ok {
							return true
						}
						diagnostics = append(diagnostics, pass.diagnostic(desc, file, deferStmt.Pos(), deferStmt.End(), "defer executed inside a loop", ""))
						return true
					})
				}
				return true
			})
		}
	}
	return diagnostics
}

func runFmtSprintSimpleConcat(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, pkg := range pass.typedPackages {
		for _, file := range pkg.files {
			ast.Inspect(file.file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if isPkgFuncCall(file.file, call.Fun, "fmt", "Sprintf") && len(call.Args) > 1 {
					format, ok := constantString(pkg, call.Args[0])
					if ok {
						plain := strings.ReplaceAll(strings.ReplaceAll(format, "%%", ""), "%s", "")
						if !strings.Contains(plain, "%") {
							allStrings := true
							for _, arg := range call.Args[1:] {
								if !isStringType(typeOf(pkg, arg)) {
									allStrings = false
									break
								}
							}
							if allStrings {
								diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, call.Pos(), call.End(), "fmt.Sprintf used for simple string concatenation", ""))
							}
						}
					}
				}
				if isPkgFuncCall(file.file, call.Fun, "fmt", "Sprint") && len(call.Args) > 1 {
					allStrings := true
					for _, arg := range call.Args {
						if !isStringType(typeOf(pkg, arg)) {
							allStrings = false
							break
						}
					}
					if allStrings {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, call.Pos(), call.End(), "fmt.Sprint used for simple string concatenation", ""))
					}
				}
				return true
			})
		}
	}
	return diagnostics
}

func runBytesBufferCopy(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, pkg := range pass.typedPackages {
		for _, file := range pkg.files {
			ast.Inspect(file.file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if isPkgFuncCall(file.file, call.Fun, "bytes", "NewBuffer") && len(call.Args) == 1 {
					if inner, ok := isConversionToByteSlice(call.Args[0]); ok && isStringType(typeOf(pkg, inner)) {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, call.Pos(), call.End(), "bytes.NewBuffer copies a string via []byte conversion", ""))
					}
				}
				if ident, ok := isMethodCall(call.Fun, "Write"); ok && len(call.Args) == 1 {
					if inner, ok := isConversionToByteSlice(call.Args[0]); ok && isStringType(typeOf(pkg, inner)) {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, call.Pos(), call.End(), "bytes.Buffer.Write copies a string via []byte conversion", ident.Name))
					}
				}
				return true
			})
		}
	}
	return diagnostics
}

func runJSONUnmarshalTwice(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.nonTest {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				seen := map[string]token.Pos{}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok || !isPkgFuncCall(file.file, call.Fun, "encoding/json", "Unmarshal") || len(call.Args) < 1 {
						return true
					}
					key := exprText(pass.fset, call.Args[0])
					if _, ok := seen[key]; ok {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file, call.Pos(), call.End(), "same JSON payload is unmarshaled multiple times in one function", fn.Name.Name))
						return true
					}
					seen[key] = call.Pos()
					return true
				})
			}
		}
	}
	return diagnostics
}

func runHTTPRequestWithoutContext(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	return runPackageCallRule(pass, desc, "net/http", []string{"NewRequest"}, "http.NewRequest used without context")
}

func runHTTPDefaultClient(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	diagnostics = append(diagnostics,
		runPackageCallRule(pass, desc, "net/http", []string{"Get", "Post", "PostForm", "Head"}, "package-level net/http client helper used")...,
	)
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.nonTest {
			ast.Inspect(file.file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok || !isPkgFuncCall(file.file, selector, "net/http", "DefaultClient") {
					return true
				}
				diagnostics = append(diagnostics, pass.diagnostic(desc, file, selector.Pos(), selector.End(), "http.DefaultClient used directly", ""))
				return true
			})
		}
	}
	return diagnostics
}

func runHTTPServerNoTimeouts(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	diagnostics = append(diagnostics,
		runPackageCallRule(pass, desc, "net/http", []string{"ListenAndServe", "ListenAndServeTLS"}, "HTTP server started without explicit timeouts")...,
	)
	for _, pkg := range pass.typedPackages {
		for _, file := range pkg.files {
			ast.Inspect(file.file, func(node ast.Node) bool {
				composite, ok := node.(*ast.CompositeLit)
				if !ok || !isNamedType(typeOf(pkg, composite), "net/http", "Server") {
					return true
				}
				hasTimeout := false
				for _, elt := range composite.Elts {
					field, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, ok := field.Key.(*ast.Ident)
					if !ok {
						continue
					}
					switch key.Name {
					case "ReadTimeout", "ReadHeaderTimeout", "WriteTimeout", "IdleTimeout":
						hasTimeout = true
					}
				}
				if !hasTimeout {
					diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, composite.Pos(), composite.End(), "http.Server created without timeout fields", ""))
				}
				return true
			})
		}
	}
	return diagnostics
}

func runTimeTickLeak(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	return runPackageCallRule(pass, desc, "time", []string{"Tick"}, "time.Tick used without a matching stop path")
}

func runTimeAfterInLoop(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.nonTest {
			ast.Inspect(file.file, func(node ast.Node) bool {
				switch loop := node.(type) {
				case *ast.ForStmt:
					if loopContainsCall(loop.Body, func(call *ast.CallExpr) bool {
						return isPkgFuncCall(file.file, call.Fun, "time", "After")
					}) {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file, loop.Pos(), loop.End(), "time.After called inside a loop", ""))
					}
				case *ast.RangeStmt:
					if loopContainsCall(loop.Body, func(call *ast.CallExpr) bool {
						return isPkgFuncCall(file.file, call.Fun, "time", "After")
					}) {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file, loop.Pos(), loop.End(), "time.After called inside a loop", ""))
					}
				}
				return true
			})
		}
	}
	return diagnostics
}

func runRowsNotClosed(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	return runDBRowsRule(pass, desc, func(varState dbRowsState) bool { return !varState.closed }, "database rows value is never closed")
}

func runRowsErrNotChecked(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	return runDBRowsRule(pass, desc, func(varState dbRowsState) bool { return varState.iterated && !varState.checkedErr }, "rows.Err is never checked after iterating rows")
}

func runTxNoDeferredRollback(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, pkg := range pass.typedPackages {
		for _, file := range pkg.files {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				positions := map[string]token.Pos{}
				rollback := map[string]bool{}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					switch typed := node.(type) {
					case *ast.AssignStmt:
						if len(typed.Lhs) == 0 || len(typed.Rhs) != 1 {
							return true
						}
						call, ok := typed.Rhs[0].(*ast.CallExpr)
						if !ok {
							return true
						}
						for _, lhs := range typed.Lhs {
							if ident, ok := lhs.(*ast.Ident); ok {
								if !isSQLTxType(objectType(pkg, ident)) {
									continue
								}
								positions[ident.Name] = call.Pos()
								break
							}
						}
					case *ast.DeferStmt:
						ident, ok := isMethodCall(typed.Call.Fun, "Rollback")
						if ok {
							if _, exists := positions[ident.Name]; exists {
								rollback[ident.Name] = true
							}
						}
					}
					return true
				})
				for name, pos := range positions {
					if rollback[name] {
						continue
					}
					diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, pos, pos, "transaction opened without a deferred rollback", fn.Name.Name))
				}
			}
		}
	}
	return diagnostics
}

func runReadAllUnbounded(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.files {
			ast.Inspect(file.file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || !isPkgFuncCall(file.file, call.Fun, "io", "ReadAll") && !isPkgFuncCall(file.file, call.Fun, "io/ioutil", "ReadAll") {
					return true
				}
				if len(call.Args) > 0 {
					if selector, ok := call.Args[0].(*ast.SelectorExpr); ok && selector.Sel.Name == "Body" {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file, call.Pos(), call.End(), "ReadAll called on a body-like reader without a size bound", ""))
					}
				}
				return true
			})
		}
	}
	return diagnostics
}

func runCrossLayerImport(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	rules := pass.target.Architecture
	useBuiltInDefaults := len(rules) == 0
	layers := make([]diagnosticLayer, 0, len(rules))
	for _, layer := range rules {
		layers = append(layers, diagnosticLayer{
			name:    layer.Name,
			include: append([]string(nil), layer.Include...),
			allow:   append([]string(nil), layer.Allow...),
		})
	}
	if len(layers) == 0 {
		layers = []diagnosticLayer{
			{name: "domain", include: []string{"internal/domain/..."}},
			{name: "platform", include: []string{"internal/platform/..."}},
			{name: "transport", include: []string{"internal/transport/..."}},
		}
	}

	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		layerName, ok := matchLayer(dir.relDir, layers)
		if !ok {
			continue
		}
		for _, file := range dir.nonTest {
			for _, spec := range file.file.Imports {
				importPath := unquoteImport(spec.Path.Value)
				importDir := pass.directoryForImport(importPath)
				if importDir == nil {
					continue
				}
				targetLayer, ok := matchLayer(importDir.relDir, layers)
				if !ok || targetLayer == layerName {
					continue
				}
				if isAllowedLayer(layerName, targetLayer, layers, useBuiltInDefaults) {
					continue
				}
				diagnostics = append(diagnostics, pass.diagnostic(desc, file, spec.Pos(), spec.End(), "import crosses a restricted architecture layer boundary", importPath))
			}
		}
	}
	return diagnostics
}

type diagnosticLayer struct {
	name    string
	include []string
	allow   []string
}

func runForbiddenPackageCycles(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	graph := map[string][]string{}
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.nonTest {
			for _, spec := range file.file.Imports {
				importPath := unquoteImport(spec.Path.Value)
				if _, ok := pass.repoImportPathSet[importPath]; !ok {
					continue
				}
				graph[dir.importPath] = append(graph[dir.importPath], importPath)
			}
		}
	}
	components := stronglyConnected(graph)
	var diagnostics []model.Diagnostic
	for _, component := range components {
		if len(component) < 2 {
			continue
		}
		componentSet := map[string]struct{}{}
		for _, importPath := range component {
			componentSet[importPath] = struct{}{}
		}
		for _, dir := range pass.directories {
			if !pass.shouldVisitDir(dir.relDir) {
				continue
			}
			if _, ok := componentSet[dir.importPath]; !ok || len(dir.nonTest) == 0 {
				continue
			}
			file := dir.nonTest[0]
			diagnostics = append(diagnostics, pass.diagnostic(desc, file, file.file.Pos(), file.file.Name.End(), "package participates in a repository import cycle", dir.importPath))
		}
	}
	return diagnostics
}

func runOversizedPackage(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) || len(dir.nonTest) == 0 {
			continue
		}
		decls := 0
		for _, file := range dir.nonTest {
			decls += len(file.file.Decls)
		}
		if len(dir.nonTest) <= oversizedPackageFileThreshold && decls <= oversizedPackageDeclThreshold {
			continue
		}
		file := dir.nonTest[0]
		diagnostics = append(diagnostics, pass.diagnostic(desc, file, file.file.Pos(), file.file.Name.End(), "package exceeds the default size threshold", dir.importPath))
	}
	return diagnostics
}

func runGodFile(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.nonTest {
			if file.lineCount <= cohesiveFileLineAllowance {
				continue
			}
			if len(file.file.Decls) <= godFileDeclThreshold && file.lineCount <= godFileLineThreshold {
				continue
			}
			diagnostics = append(diagnostics, pass.diagnostic(desc, file, file.file.Pos(), file.file.End(), "file exceeds the default god-file threshold", path.Base(file.relPath)))
		}
	}
	return diagnostics
}

func runMissingTableDriven(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.tests {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Name == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
					continue
				}
				hasRange := false
				calls := map[string]int{}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					switch typed := node.(type) {
					case *ast.RangeStmt:
						hasRange = true
					case *ast.CallExpr:
						calls[exprText(pass.fset, typed.Fun)]++
					}
					return true
				})
				if hasRange {
					continue
				}
				for _, count := range calls {
					if count >= 3 {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file, fn.Pos(), fn.Name.End(), "test repeats the same call pattern without using a table", fn.Name.Name))
						break
					}
				}
			}
		}
	}
	return diagnostics
}

func runNoAssertions(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.tests {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Name == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
					continue
				}
				hasAssertion := false
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					selector, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					switch selector.Sel.Name {
					case "Fatal", "Fatalf", "Error", "Errorf", "Fail", "FailNow":
						hasAssertion = true
						return false
					}
					return true
				})
				if !hasAssertion {
					diagnostics = append(diagnostics, pass.diagnostic(desc, file, fn.Pos(), fn.Name.End(), "test does not contain an assertion or failure path", fn.Name.Name))
				}
			}
		}
	}
	return diagnostics
}

func runSleepInTest(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.tests {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Name == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
					continue
				}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok || !isPkgFuncCall(file.file, call.Fun, "time", "Sleep") {
						return true
					}
					diagnostics = append(diagnostics, pass.diagnostic(desc, file, call.Pos(), call.End(), "test uses time.Sleep", fn.Name.Name))
					return true
				})
			}
		}
	}
	return diagnostics
}

func runHTTPHandlerNoTest(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, pkg := range pass.typedPackages {
		testContent := map[string]bool{}
		if pkg.dir != nil {
			for _, file := range pkg.dir.tests {
				ast.Inspect(file.file, func(node ast.Node) bool {
					ident, ok := node.(*ast.Ident)
					if ok {
						testContent[ident.Name] = true
					}
					return true
				})
			}
		}
		for _, file := range pkg.files {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name == nil || fn.Body == nil {
					continue
				}
				signature := funcDeclSignature(pkg, fn)
				if signature == nil || signature.Params().Len() != 2 {
					continue
				}
				if !isHTTPResponseWriterType(signature.Params().At(0).Type()) || !isHTTPRequestType(signature.Params().At(1).Type()) {
					continue
				}
				if testContent[fn.Name.Name] {
					continue
				}
				diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, fn.Pos(), fn.Name.End(), "HTTP handler has no matching test reference", fn.Name.Name))
			}
		}
	}
	return diagnostics
}

func runMathRandForSecret(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	keywords := []string{"token", "secret", "password", "passwd", "key", "auth", "session", "nonce"}
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.files {
			ast.Inspect(file.file, func(node ast.Node) bool {
				switch typed := node.(type) {
				case *ast.AssignStmt:
					if len(typed.Lhs) == 0 || len(typed.Rhs) != 1 {
						return true
					}
					call, ok := typed.Rhs[0].(*ast.CallExpr)
					if !ok || !isPkgFuncCall(file.file, call.Fun, "math/rand", "Int", "Int31", "Int63", "Intn", "Read", "Uint32", "Uint64") {
						return true
					}
					for _, lhs := range typed.Lhs {
						ident, ok := lhs.(*ast.Ident)
						if ok && containsKeyword(ident.Name, keywords) {
							diagnostics = append(diagnostics, pass.diagnostic(desc, file, call.Pos(), call.End(), "math/rand used while generating secret-like data", ident.Name))
							return false
						}
					}
				case *ast.FuncDecl:
					if typed.Name != nil && containsKeyword(typed.Name.Name, keywords) && typed.Body != nil {
						ast.Inspect(typed.Body, func(child ast.Node) bool {
							call, ok := child.(*ast.CallExpr)
							if !ok || !isPkgFuncCall(file.file, call.Fun, "math/rand", "Int", "Int31", "Int63", "Intn", "Read", "Uint32", "Uint64") {
								return true
							}
							diagnostics = append(diagnostics, pass.diagnostic(desc, file, call.Pos(), call.End(), "math/rand used inside a secret-generation code path", typed.Name.Name))
							return false
						})
					}
				}
				return true
			})
		}
	}
	return diagnostics
}

func runInsecureTempFile(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.files {
			ast.Inspect(file.file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || !isPkgFuncCall(file.file, call.Fun, "os", "Create", "OpenFile") {
					return true
				}
				if len(call.Args) > 0 && pathUsesTempDir(file.file, call.Args[0]) {
					diagnostics = append(diagnostics, pass.diagnostic(desc, file, call.Pos(), call.End(), "predictable temporary file path created without os.CreateTemp", ""))
				}
				return true
			})
		}
	}
	return diagnostics
}

func runExecUserInput(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.files {
			ast.Inspect(file.file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || !isPkgFuncCall(file.file, call.Fun, "os/exec", "Command", "CommandContext") {
					return true
				}
				start := 1
				if isPkgFuncCall(file.file, call.Fun, "os/exec", "CommandContext") {
					start = 2
				}
				for _, arg := range call.Args[start:] {
					if isUserInputExpr(file.file, arg) {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file, call.Pos(), call.End(), "os/exec command receives direct user input", ""))
						break
					}
				}
				return true
			})
		}
	}
	return diagnostics
}

func runErrorStringBranching(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.files {
			ast.Inspect(file.file, func(node ast.Node) bool {
				switch typed := node.(type) {
				case *ast.SwitchStmt:
					if typed.Tag != nil && isErrorStringExpr(typed.Tag) {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file, typed.Pos(), typed.End(), "switch branches on err.Error()", ""))
					}
				case *ast.CallExpr:
					if !isPkgFuncCall(file.file, typed.Fun, "strings", "Contains", "HasPrefix", "HasSuffix") || len(typed.Args) == 0 {
						return true
					}
					if isErrorStringExpr(typed.Args[0]) {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file, typed.Pos(), typed.End(), "string matching performed on err.Error()", ""))
					}
				}
				return true
			})
		}
	}
	return diagnostics
}

func runExportedMutableGlobal(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.nonTest {
			for _, decl := range file.file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.VAR {
					continue
				}
				for _, spec := range gen.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for index, name := range valueSpec.Names {
						if !name.IsExported() {
							continue
						}
						if strings.HasPrefix(name.Name, "Err") && len(valueSpec.Values) > index {
							if call, ok := valueSpec.Values[index].(*ast.CallExpr); ok && (isPkgFuncCall(file.file, call.Fun, "errors", "New") || isPkgFuncCall(file.file, call.Fun, "fmt", "Errorf")) {
								continue
							}
						}
						diagnostics = append(diagnostics, pass.diagnostic(desc, file, name.Pos(), name.End(), "exported package variable exposes mutable global state", name.Name))
					}
				}
			}
		}
	}
	return diagnostics
}

func runInitSideEffects(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.files {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name == nil || fn.Name.Name != "init" || fn.Body == nil {
					continue
				}
				reported := false
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					if reported {
						return false
					}
					switch typed := node.(type) {
					case *ast.GoStmt:
						reported = true
						diagnostics = append(diagnostics, pass.diagnostic(desc, file, typed.Pos(), typed.End(), "init launches a goroutine", "init"))
						return false
					case *ast.CallExpr:
						if isPkgFuncCall(file.file, typed.Fun, "flag", "Parse") ||
							isPkgFuncCall(file.file, typed.Fun, "os", "Setenv", "Chdir") ||
							isPkgFuncCall(file.file, typed.Fun, "net/http", "Handle", "HandleFunc") ||
							isPkgFuncCall(file.file, typed.Fun, "database/sql", "Open") ||
							isPkgFuncCall(file.file, typed.Fun, "net", "Listen") ||
							isPkgFuncCall(file.file, typed.Fun, "os/exec", "Command") ||
							isPkgFuncCall(file.file, typed.Fun, "time", "NewTicker", "Tick") {
							reported = true
							diagnostics = append(diagnostics, pass.diagnostic(desc, file, typed.Pos(), typed.End(), "init performs process or I/O side effects", "init"))
							return false
						}
					}
					return true
				})
			}
		}
	}
	return diagnostics
}

func runOSExitInNonMain(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.files {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok || !isPkgFuncCall(file.file, call.Fun, "os", "Exit") {
						return true
					}
					if !isMainFunction(file, fn) {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file, call.Pos(), call.End(), "os.Exit called outside package main's main function", fn.Name.Name))
					}
					return true
				})
			}
		}
	}
	return diagnostics
}

func runFlagParseInNonMain(pass *analysisContext, desc Descriptor) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.files {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok || !isPkgFuncCall(file.file, call.Fun, "flag", "Parse") {
						return true
					}
					if !isMainFunction(file, fn) {
						diagnostics = append(diagnostics, pass.diagnostic(desc, file, call.Pos(), call.End(), "flag.Parse called outside package main's main function", fn.Name.Name))
					}
					return true
				})
			}
		}
	}
	return diagnostics
}

type dbRowsState struct {
	pos        token.Pos
	closed     bool
	iterated   bool
	checkedErr bool
}

func runDBRowsRule(pass *analysisContext, desc Descriptor, needsReport func(dbRowsState) bool, message string) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, pkg := range pass.typedPackages {
		for _, file := range pkg.files {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				rows := map[string]dbRowsState{}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					switch typed := node.(type) {
					case *ast.AssignStmt:
						if len(typed.Rhs) != 1 || len(typed.Lhs) == 0 {
							return true
						}
						call, ok := typed.Rhs[0].(*ast.CallExpr)
						if !ok {
							return true
						}
						for _, lhs := range typed.Lhs {
							ident, ok := lhs.(*ast.Ident)
							if ok {
								if !isSQLRowsType(objectType(pkg, ident)) {
									continue
								}
								rows[ident.Name] = dbRowsState{pos: call.Pos()}
								break
							}
						}
					case *ast.CallExpr:
						if ident, ok := isMethodCall(typed.Fun, "Close"); ok {
							if _, exists := rows[ident.Name]; !exists {
								return true
							}
							state := rows[ident.Name]
							state.closed = true
							rows[ident.Name] = state
						}
						if ident, ok := isMethodCall(typed.Fun, "Next"); ok {
							if _, exists := rows[ident.Name]; !exists {
								return true
							}
							state := rows[ident.Name]
							state.iterated = true
							rows[ident.Name] = state
						}
						if ident, ok := isMethodCall(typed.Fun, "Err"); ok {
							if _, exists := rows[ident.Name]; !exists {
								return true
							}
							state := rows[ident.Name]
							state.checkedErr = true
							rows[ident.Name] = state
						}
					}
					return true
				})
				for name, state := range rows {
					if !needsReport(state) {
						continue
					}
					diagnostics = append(diagnostics, pass.diagnostic(desc, file.source, state.pos, state.pos, message, name))
				}
			}
		}
	}
	return diagnostics
}

func runMethodResourceRule(pass *analysisContext, desc Descriptor, importPath string, ctor string, cleanup string, message string) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.nonTest {
			for _, decl := range file.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				positions := map[string]token.Pos{}
				cleaned := map[string]bool{}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					switch typed := node.(type) {
					case *ast.AssignStmt:
						if len(typed.Rhs) != 1 {
							return true
						}
						call, ok := typed.Rhs[0].(*ast.CallExpr)
						if !ok || !isPkgFuncCall(file.file, call.Fun, importPath, ctor) {
							return true
						}
						for _, lhs := range typed.Lhs {
							if ident, ok := lhs.(*ast.Ident); ok {
								positions[ident.Name] = call.Pos()
								break
							}
						}
					case *ast.CallExpr:
						if ident, ok := isMethodCall(typed.Fun, cleanup); ok {
							if _, exists := positions[ident.Name]; exists {
								cleaned[ident.Name] = true
							}
						}
					}
					return true
				})
				for name, pos := range positions {
					if cleaned[name] {
						continue
					}
					diagnostics = append(diagnostics, pass.diagnostic(desc, file, pos, pos, message, name))
				}
			}
		}
	}
	return diagnostics
}

func runPackageCallRule(pass *analysisContext, desc Descriptor, importPath string, names []string, message string) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, dir := range pass.directories {
		if !pass.shouldVisitDir(dir.relDir) {
			continue
		}
		for _, file := range dir.files {
			ast.Inspect(file.file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || !isPkgFuncCall(file.file, call.Fun, importPath, names...) {
					return true
				}
				diagnostics = append(diagnostics, pass.diagnostic(desc, file, call.Pos(), call.End(), message, ""))
				return true
			})
		}
	}
	return diagnostics
}

func containsKeyword(value string, keywords []string) bool {
	lower := strings.ToLower(value)
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func isUserInputExpr(file *ast.File, expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.IndexExpr:
		return isPkgFuncCall(file, typed.X, "os", "Args")
	case *ast.CallExpr:
		return isPkgFuncCall(file, typed.Fun, "flag", "Arg", "Args") ||
			isPkgFuncCall(file, typed.Fun, "os", "Getenv") ||
			isPkgFuncCall(file, typed.Fun, "net/http", "FormValue", "PostFormValue") ||
			isRequestQueryGet(typed)
	case *ast.SelectorExpr:
		return typed.Sel.Name == "FormValue" || typed.Sel.Name == "PostFormValue"
	}
	return false
}

func isRequestQueryGet(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Get" {
		return false
	}
	queryCall, ok := selector.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	querySelector, ok := queryCall.Fun.(*ast.SelectorExpr)
	return ok && querySelector.Sel.Name == "Query"
}

func (pass *analysisContext) directoryForImport(importPath string) *directoryInfo {
	for _, dir := range pass.directories {
		if dir.importPath == importPath {
			return dir
		}
	}
	return nil
}

func matchLayer(relDir string, rules []diagnosticLayer) (string, bool) {
	normalized := model.NormalizePath(relDir)
	for _, rule := range rules {
		for _, include := range rule.include {
			if layerMatch(include, normalized) {
				return rule.name, true
			}
		}
	}
	return "", false
}

func isAllowedLayer(from string, to string, rules []diagnosticLayer, useBuiltInDefaults bool) bool {
	for _, rule := range rules {
		if rule.name != from {
			continue
		}
		for _, allow := range rule.allow {
			if allow == to {
				return true
			}
		}
		if len(rule.allow) > 0 {
			return false
		}
		break
	}
	if useBuiltInDefaults && (from == "domain" || from == "platform") {
		return to != "transport"
	}
	return true
}

func layerMatch(pattern string, value string) bool {
	pattern = strings.TrimPrefix(pattern, "./")
	value = strings.TrimPrefix(value, "./")
	if strings.HasSuffix(pattern, "/...") {
		base := strings.TrimSuffix(pattern, "/...")
		return value == base || strings.HasPrefix(value, base+"/")
	}
	return pattern == value
}

func stronglyConnected(graph map[string][]string) [][]string {
	index := 0
	stack := []string{}
	onStack := map[string]bool{}
	indices := map[string]int{}
	low := map[string]int{}
	var components [][]string

	var visit func(string)
	visit = func(node string) {
		indices[node] = index
		low[node] = index
		index++
		stack = append(stack, node)
		onStack[node] = true

		for _, next := range graph[node] {
			if _, ok := indices[next]; !ok {
				visit(next)
				if low[next] < low[node] {
					low[node] = low[next]
				}
				continue
			}
			if onStack[next] && indices[next] < low[node] {
				low[node] = indices[next]
			}
		}

		if low[node] != indices[node] {
			return
		}
		var component []string
		for {
			last := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[last] = false
			component = append(component, last)
			if last == node {
				break
			}
		}
		components = append(components, component)
	}

	for node := range graph {
		if _, ok := indices[node]; ok {
			continue
		}
		visit(node)
	}
	return components
}
