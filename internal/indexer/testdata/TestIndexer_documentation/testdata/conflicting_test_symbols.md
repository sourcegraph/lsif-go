# Package osl

## Index

* [Variables](#var)
    * [var ErrNotImplemented](#ErrNotImplemented)
* [Functions](#func)
    * [func GenerateKey(containerID string) string](#GenerateKey)
    * [func NewSandbox(key string, osCreate, isRestore bool) (Sandbox, error)](#NewSandbox)


## <a id="var" href="#var">Variables</a>

```
tags: [private]
```

### <a id="ErrNotImplemented" href="#ErrNotImplemented">var ErrNotImplemented</a>

```
searchKey: osl.ErrNotImplemented
tags: [variable interface]
```

```Go
var ErrNotImplemented = errors.New("not implemented")
```

ErrNotImplemented is for platforms which don't implement sandbox 

## <a id="func" href="#func">Functions</a>

```
tags: [private]
```

### <a id="GenerateKey" href="#GenerateKey">func GenerateKey(containerID string) string</a>

```
searchKey: osl.GenerateKey
tags: [function]
```

```Go
func GenerateKey(containerID string) string
```

GenerateKey generates a sandbox key based on the passed container id. 

### <a id="NewSandbox" href="#NewSandbox">func NewSandbox(key string, osCreate, isRestore bool) (Sandbox, error)</a>

```
searchKey: osl.NewSandbox
tags: [function]
```

```Go
func NewSandbox(key string, osCreate, isRestore bool) (Sandbox, error)
```

NewSandbox provides a new sandbox instance created in an os specific way provided a key which uniquely identifies the sandbox 

