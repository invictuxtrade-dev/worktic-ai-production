// Go version independent AST inspection of the complete source tree.
// This is only a parser test; it does NOT replace `go test ./...` or `go build ./...`.
package main
import("fmt";"go/parser";"go/token";"os";"path/filepath")
func main(){
 if len(os.Args)<2 {fmt.Fprintln(os.Stderr,"usage: go run go_syntax_scan.go -- PROJECT_ROOT");os.Exit(2)}
 root:=os.Args[len(os.Args)-1]
 fs,e:=filepath.Glob(filepath.Join(root,"*.go"));if e!=nil{panic(e)}
 if len(fs)<50{fmt.Fprintln(os.Stderr,"Missing application Go sources");os.Exit(1)}
 fset:=token.NewFileSet()
 for _,f:=range fs {if _,e:=parser.ParseFile(fset,f,nil,parser.AllErrors);e!=nil {fmt.Fprintln(os.Stderr,e);os.Exit(1)}}
 fmt.Printf("AST_VALID %d Go files (NOT COMPILED)\n",len(fs))
}
