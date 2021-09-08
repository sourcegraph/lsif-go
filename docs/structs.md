# Structs

Structs are generally implemented in a relatively straightforward way.

For example:

```go
type MyStruct struct {
    Cli http.Client
    ^^^----------------- definition MyStruct.Cli
        ^^^^------------ reference github.com/golang/go/std/http
             ^^^^^^----- reference github.com/golang/go/std/http.Client
}

```

But, for anonymous fields, it is a little more complicated, and ends up looking something like this.

```go
type NestedHandler struct {
    LocalItem
    ^^^^^^^^^-------- definition MyStruct.LocalItem
    ^^^^^^^^^-------- reference LocalItem
}
```

In this case it is possible to have the same ranges overlapping, so `lsif-go`
will re-use the same range.

However, in the following case, we have three separate ranges that, while they overlap
are not identical, so they cannot be shared and a new range must be created.

```go
type Nested struct {
    http.Handler
    ^^^^^^^^^^^^-------- definition Nested.Handler
    ^^^^---------------- reference github.com/golang/go/std/http
         ^^^^^^^-------- reference github.com/golang/go/std/http.Handler
}
```
