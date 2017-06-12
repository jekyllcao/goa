 v2

This branch is work in progress for goa v2.

v2 brings a host of fixes and has a cleaner more composable overall design. Most
notably the DSL engine assumes less about the DSL and is thus more generic. The
top level design package is also hugely simplified to focus solely on the core
DSL expressions.

The import path for goa v2 has changed from `github.com/goadesign/goa` to
`goa.design/goa.v2`.

## Multiple Transport Support

The new top level `rest` package implement the DSL, design objects, code
generation and runtime support for HTTP/REST APIs. The REST DSL is built on top
of the core DSL package and add transport specific keywords to describe aspects
specific to HTTP requests and responses. This structure makes it possible to add
support for other transports especially gRPC.
See
[goa v2 Design Requirements](https://github.com/goadesign/goa/blob/v2/REQ_DESIGN.md).

## New Data Types

The primitive types now include `Int`, `Int32`, `Int64`, `UInt` `UInt32`,
`UInt64`, `Float32`, `Float64` and `Bytes`. This makes it possible to support
transports such as gRPC but also makes REST interface definitions crisper. The
v1 types `Integer` and `Float` have been removed in favor of these new types.

## Composable Code Generation

Code generation now follows a 2-phase process where the first phase produces a
set of writers each exposing templates that can be further modified prior to
running the last phase which generates the final artefacts. This makes it
possible for plugins to alter the code generated by the built-in code
generators, see
the
[Code Generation Requirements](https://github.com/goadesign/goa/blob/v2/REQ_CODEGEN.md).

## Separation of Concern

The generated code produced by `goagen` v2 implements a much stronger separation
of concern between the transport and service layers. This makes it possible to
easily expose the same methods via different transport mechanisms such as HTTP
and gRPC. See the
[account example](https://github.com/goadesign/goa/tree/v2/examples/account)
which illustrates the new generated code structure.

## Getting Started

v2 is work in progress. The code is **not** in a usable state (in particular the
code generators are not done yet). However:

* The core and rest DSLs should be fairly stable, see:
  - The [core API DSL spec](https://github.com/goadesign/goa/blob/v2/dsl/_spec/dsl_spec_test.go)
  - The [type DSL spec](https://github.com/goadesign/goa/blob/v2/dsl/_spec/type_spec_test.go)
  - The [rest DSL spec](https://github.com/goadesign/goa/blob/v2/dsl/rest/_spec/dsl_spec_test.go)

* The account example should give a good sense of what the generated code will
  look like, see:
  - The [design](https://github.com/goadesign/goa/blob/v2/examples/account/design/design.go)
  - The [main scaffold](https://github.com/goadesign/goa/blob/v2/examples/account/cmd/basic/main.go)
  - The [generated service](https://github.com/goadesign/goa/blob/v2/examples/account/gen/service/account.go)
  - The [generated endpoints](https://github.com/goadesign/goa/blob/v2/examples/account/gen/endpoints/account.go)
  - The [generated HTTP transport](https://github.com/goadesign/goa/tree/v2/examples/account/gen/transport/http)

## Getting Involved

Ping us on [slack](https://gophers.slack.com/messages/goa/) or open an issue
tagged with `v2` if you have feedback on the above or would like to contribute.
One area that might be interesting to look at may be to write what the
generated code would be for the gRPC transport.

## Design Doc Tasks

- [ ] DSL layout (API -> Service -> Method -> Payload/Result/Errors)
- [ ] Types: primitives
- [ ] Types: arrays
- [ ] Types: maps
- [ ] Types: objects
- [ ] Types: user types
- [ ] Types: result types
- [ ] Validations
- [x] Transport DSL layout: HTTP
- [x] Payload to HTTP request mapping: non-object types
- [x] Payload to HTTP request mapping: object types
- [ ] Result to HTTP response mapping: non-object types
- [ ] Result to HTTP response mapping: object types

### Payload to HTTP request mapping

The payload types describe the shape of the data given as argument to the
service methods. The HTTP transport specific DSL defines how the data is built
from the incoming HTTP request state.

The HTTP request state comprises four different parts:

- The URL path parameters (for example the route `/bottle/{id}` defines the `id` path parameter)
- The URL query string parameters
- The HTTP headers
- And finally the HTTP request body
 
The HTTP expressions drive how the generated code decodes the request into the
payload type:

* The `Param` expression defines values loaded from path or query string
  parameters.
* The `Header` expression defines values loaded from HTTP headers.
* The `Body` expression defines values loaded from the request body.


The next two sections describe the expressions in more details. 

Note that the generated code provides a default decoder implementation that
ought to be sufficient in most cases however it also makes it possible to plug a
user provided decoder in the (hopefully rare) cases when that's needed.
 
#### Mapping payload with non-object types

When the payload type is a primitive, an array or a map then the value is loaded from:

- the first URL path parameter if any
- otherwise the first query string parameter if any
- otherwise the first header if any
- otherwise the body

with the following restrictions:

- only primitive or array types may be used to define path parameters or headers
- only primitive, array and map types may be used to define query string parameters
- array and map types used to define path parameters, query string parameters or
  headers must use primitive types to define their elements

Arrays in paths and headers are represented using comma separated values.

Examples:

* simple "get by identifier" where identifiers are integers:

```go
Method("show", func() {
    Payload(Int)
    HTTP(func() {
        GET("/{id}")
    })
})
```

| Generated method | Example request | Corresponding call |
| ---------------- | --------------- | ------------------ |
| Show(int)        | GET /1          | Show(1)            |

* bulk "delete by identifiers" where identifiers are strings:

```go
Method("delete", func() {
    Payload(ArrayOf(String))
    HTTP(func() {
        DELETE("/{ids}")
    })
})
```

| Generated method   | Example request | Corresponding call         |
| ------------------ | --------------- | -------------------------- |
| Delete([]string)   | DELETE /a,b     | Delete([]string{"a", "b"}) |


> Note that in both the previous examples the name of the parameter path is
> irrelevant.

* list with filters:

```go
Method("list", func() {
    Payload(ArrayOf(String))
    HTTP(func() {
        GET("")
        Param("filter")
    })
})
```

| Generated method | Example request         | Corresponding call       |
| ---------------- | ----------------------- | ------------------------ |
| List([]string)   | GET /?filter=a&filter=b | List([]string{"a", "b"}) |

list with version:

```go
Method("list", func() {
    Payload(Float32)
    HTTP(func() {
        GET("")
        Header("version")
    })
})
```

| Generated method | Example request     | Corresponding call |
| ---------------- | ------------------- | ------------------ |
| List(float32)    | GET / [version=1.0] | List(1.0)          |

creation:

```go
Method("create", func() {
    Payload(MapOf(String, Int))
    HTTP(func() {
        POST("")
    })
})
```

| Generated method       | Example request         | Corresponding call                     |
| ---------------------- | ----------------------- | -------------------------------------- |
| Create(map[string]int) | POST / {"a": 1, "b": 2} | Create(map[string]int{"a": 1, "b": 2}) |

#### Mapping payload with object types

The HTTP expressions describe how the payload object attributes are loaded from
the HTTP request state. Different attributes may be loaded from different parts
of the request: some attributes may be loaded from the request path, some from
the query string parameters and others from the body for example. The same type
restrictions apply to the path, query string and header attributes (attributes
describing path and headers must be primitives or arrays of primitives and
attributes describing query string parameters must be primitives, arrays or maps
of primitives).

The `Body` expression makes it possible to define the payload type attribute
that describes the request body. Alternatively if the `Body` expression is
omitted then all attributes that make up the payload type and that are not used
to define a path parameter, a query string parameter or a header implicitly
describe the body.

For example, given the payload:

```go
Method("create", func() {
    Payload(func() {
        Attribute("id", Int)
        Attribute("name", String)
        Attribute("age", Int)
    })
})
```

The following HTTP expression causes the `id` attribute to get loaded from the
path parameter while `name` and `age` are loaded from the request body:

```go 
Method("create", func() {
    Payload(func() {
        Attribute("id", Int)
        Attribute("name", String)
        Attribute("age", Int)
    })
    HTTP(func() {
        POST("/{id}")
    })
})
```

| Generated method       | Example request                 | Corresponding call                               |
| ---------------------- | ------------------------------- | ------------------------------------------------ |
| Create(*CreatePayload) | POST /1 {"name": "a", "age": 2} | Create(&CreatePayload{ID: 1, Name: "a", Age: 2}) |

`Body` makes it possible to describe request bodies that are not objects such as
arrays or maps.

Consider the following payload:

```go 
Method("rate", func() {
    Payload(func() {
        Attribute("id", Int)
        Attribute("rates", MapOf(String, Float64))
    })
})
```

Using the following HTTP expression the rates are loaded from the body:

```go 
Method("rate", func() {
    Payload(func() {
        Attribute("id", Int)
        Attribute("rates", MapOf(String, Float64))
    })
    HTTP(func() {
        PUT("/{id}")
        Body("rates")
    })
})
```

| Generated method   | Example request             | Corresponding call                                                       |
| ------------------ | --------------------------- | ------------------------------------------------------------------------ |
| Rate(*RatePayload) | PUT /1 {"a": 0.5, "b": 1.0} | Rate(&RatePayload{ID: 1, Rates: map[string]float64{"a": 0.5, "b": 1.0}}) |

Without `Body` the request body shape would be an object with one key `rates`.

#### Mapping HTTP element names to attribute names

The expressions used to describe the HTTP request elements `Param`, `Header` and
`Body` may provide a mapping between the names of the elements (query string
key, header name or body field name) and the corresponding payload attribute
name. The mapping is defined using the syntax `"attribute name:element name"`,
for example:

```go 
Header("version:X-Api-Version")
```

causes the `version` attribute value to get loaded from the `X-Api-Version` HTTP
header.

The `Body` expression supports an alternative syntax where the attributes that
make up the body can be explicitly listed. This syntax allows for specifying a
mapping between the incoming data field names and the payload attribute names,
for example:

```go 
Method("create", func() {
    Payload(func() {
        Attribute("name", String)
        Attribute("age", Int)
    })
    HTTP(func() {
        POST("")
        Body(func() {
        	Attribute("name:n")
        	Attribute("age:a")
        })
    })
})
```

| Generated method       | Example request            | Corresponding call                               |
| ---------------------- | -------------------------- | ------------------------------------------------ |
| Create(*CreatePayload) | POST /1 {"n": "a", "a": 2} | Create(&CreatePayload{ID: 1, Name: "a", Age: 2}) |
