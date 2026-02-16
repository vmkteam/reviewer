# rpcgen: JSON-RPC 2.0 Client Generator Implementation for zenrpc

[![Go Report Card](https://goreportcard.com/badge/github.com/vmkteam/rpcgen)](https://goreportcard.com/report/github.com/vmkteam/rpcgen) [![Go Reference](https://pkg.go.dev/badge/github.com/vmkteam/rpcgen.svg)](https://pkg.go.dev/github.com/vmkteam/rpcgen)

`rpcgen` is a JSON-RPC 2.0 client library generator for [zenrpc](https://github.com/vmkteam/zenrpc). It supports client generation for following languages:
- Dart
- Golang
- PHP
- TypeScript
- Swift
- Kotlin
- OpenRPC schema

## Examples

### Basic usage

```go
package main

import (
	"fmt"
	"log"

	"github.com/vmkteam/rpcgen/v2"
	"github.com/vmkteam/zenrpc/v2"
)

func main() {
	rpc := zenrpc.NewServer(zenrpc.Options{})

	generated, err := rpcgen.FromSMD(rpc.SMD()).GoClient().Generate()
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("%s", generated)
}
```

### Generate in HTTP handler

```go
package main

import (
	"net/http"

	"github.com/vmkteam/rpcgen/v2"
	"github.com/vmkteam/rpcgen/v2/dart"
	"github.com/vmkteam/rpcgen/v2/golang"
	"github.com/vmkteam/rpcgen/v2/swift"
	"github.com/vmkteam/zenrpc/v2"
)

func main() {
	rpc := zenrpc.NewServer(zenrpc.Options{})

	gen := rpcgen.FromSMD(rpc.SMD())

	http.HandleFunc("/client.go", rpcgen.Handler(gen.GoClient(golang.Settings{})))
	http.HandleFunc("/client.ts", rpcgen.Handler(gen.TSClient(nil)))
	http.HandleFunc("/RpcClient.php", rpcgen.Handler(gen.PHPClient("")))
	http.HandleFunc("/client.swift", rpcgen.Handler(gen.SwiftClient(swift.Settings{})))
	http.HandleFunc("/client.dart", rpcgen.Handler(gen.DartClient(dart.Settings{ Part: "client"})))
}
```

### Add custom TypeScript type mapper

```go
package main

import (
	"fmt"
	"net/http"

	"github.com/vmkteam/rpcgen/v2"
	"github.com/vmkteam/rpcgen/v2/typescript"
	"github.com/vmkteam/zenrpc/v2"
	"github.com/vmkteam/zenrpc/v2/smd"
)

func main() {
	rpc := zenrpc.NewServer(zenrpc.Options{})

	gen := rpcgen.FromSMD(rpc.SMD())

	typeMapper := func(in smd.JSONSchema, tsType typescript.Type) typescript.Type {
		if in.Type == "object" {
			if in.Description == "Group" && in.Name == "groups" {
				tsType.Type = fmt.Sprintf("Record<number, I%s>", in.Description)
			}
		}
		
		return tsType
	}

	http.HandleFunc("/client.ts", rpcgen.Handler(gen.TSClient(typeMapper)))
}
```

### Add custom Swift type mapper

```go
package main

import (
	"fmt"
	"net/http"

	"github.com/vmkteam/rpcgen/v2"
	"github.com/vmkteam/rpcgen/v2/swift"
	"github.com/vmkteam/zenrpc/v2"
	"github.com/vmkteam/zenrpc/v2/smd"
)

func main() {
	rpc := zenrpc.NewServer(zenrpc.Options{})

	gen := rpcgen.FromSMD(rpc.SMD())

	typeMapper := func(typeName string, in smd.Property, param swift.Parameter) swift.Parameter {
		switch typeName {
		case "Group":
			switch in.Name {
			case "groups":
				param.Type = fmt.Sprintf("[Int: %s]", param.Type)
				param.DecodableDefault = swift.DefaultMap
			}
		}
		return param
	}

	http.HandleFunc("/client.swift", rpcgen.Handler(gen.SwiftClient(swift.Settings{"", typeMapper})))
}
```
### Generate Swift networking protocols 

```go
package main

import (
	"net/http"

	"github.com/vmkteam/rpcgen/v2"
	"github.com/vmkteam/rpcgen/v2/swift"
	"github.com/vmkteam/zenrpc/v2"
	"github.com/vmkteam/zenrpc/v2/smd"
)

func main() {
	rpc := zenrpc.NewServer(zenrpc.Options{})

	gen := rpcgen.FromSMD(rpc.SMD())

	http.HandleFunc("/networking.generated.swift", rpcgen.Handler(gen.SwiftClient(swift.Settings{IsProtocol: true})))
}
```

### Generate Kotlin networking protocols, rpc models and custom type mapper
Kotlin settings have a lot of properties:
- Class - custom interface name. Default value: `kotlin.BaseClass`
- PackageAPI - custom package name. Default value: `kotlin.BasePackageAPI`
- Imports - optional list of imports in interface.
- IsProtocol - flag controls the output type. Set to `true` to generate a Kotlin interface for the JSON-RPC client, or set to `false` to generate the corresponding data class models.
- TypeMapper - function allows you to implement custom logic for converting schema types into specific Kotlin types.
```go
package main

import (
	"fmt"
	"net/http"

	"github.com/vmkteam/rpcgen/v2"
	"github.com/vmkteam/rpcgen/v2/kotlin"
	"github.com/vmkteam/zenrpc/v2"
	"github.com/vmkteam/zenrpc/v2/smd"
)

func main() {
	rpc := zenrpc.NewServer(zenrpc.Options{})

	gen := rpcgen.FromSMD(rpc.SMD())

	kotlinTypeMapper := func(typeName string, in smd.Property, param kotlin.Parameter) kotlin.Parameter {
		switch typeName {
		case "CustomMapType":
				param.Type = "Map<String, String>"
				param.DecodableDefault = kotlin.DefaultMap
		case "Group":
			if in.Name == "group"{
				param.Type = fmt.Sprintf("Map<Int, %s>", param.Type)
				param.DecodableDefault = kotlin.DefaultMap
			}
		}
		return param
	}
	
	http.HandleFunc("/networking.generated.kt", rpcgen.Handler(gen.KotlinClient(kotlin.Settings{PackageAPI: "example.api", IsProtocol: true, TypeMapper: kotlinTypeMapper})))
	http.HandleFunc("/rpc.generated.kt", rpcgen.Handler(gen.KotlinClient(kotlin.Settings{Class: "ExampleApi", TypeMapper: kotlinTypeMapper, PackageAPI: "example.api", Imports: []string{"exampleImport"}})))
}
```

### Add custom Dart type mapper

```go
package main

import (
	"net/http"

	"github.com/vmkteam/rpcgen/v2"
	"github.com/vmkteam/rpcgen/v2/dart"
	"github.com/vmkteam/zenrpc/v2"
	"github.com/vmkteam/zenrpc/v2/smd"
)

func main() {
	rpc := zenrpc.NewServer(zenrpc.Options{})

	gen := rpcgen.FromSMD(rpc.SMD())

	typeMapper := func(in smd.JSONSchema, param dart.Parameter) dart.Parameter {
		if in.Type == smd.Object {
			switch in.TypeName {
			case "Time", "Date":
				param.Type = "String"
			}
		}
		if in.Type == smd.Array {
			switch in.TypeName {
			case "[]Date", "[]Time":
				param.Type = "List<String>"
				param.ReturnType = "List<String>"
			}
		}
		
		return param
	}

	http.HandleFunc("/client.dart", rpcgen.Handler(gen.DartClient(dart.Settings{Part: "client", TypeMapper: typeMapper})))
}
```
