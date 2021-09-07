# Imports

There are two types of imports available in Go. In both cases, we generate the same reference
to the package itself. This is done by creating an importMoniker. This import moniker

```go
import "fmt"
//      ^^^------ reference github.com/golang/go/std/fmt

import f "fmt"
//     ^--------- local definition
//        ^^^---- reference github.com/golang/go/std/fmt


// Special Case, "." generates no local def
import . "fmt"
//                no local def
//        ^^^---- reference github.com/golang/go/std/fmt
```

## Example

So given this kind of import, you will see the following.

```go
import (
	"fmt"
	. "net/http"
	s "sort"
)
```

- Regular `"fmt"` import. Creates only a reference to the moniker

![fmt_import](/docs/media/fmt_import.png)

- Named `s "sort"` import. Creates both a reference and a definition. Any local
references to `s` in this case will link back to the definition of this import.
`"sort"` will still link to the external package.

![sort_import](/docs/media/sort_import.png)

![s_definition](/docs/media/s_definition.png)

- `.` import. This will also only create a reference, because `.` does not
create a new definition. It just pulls it into scope.

![http_import](/docs/media/http_import.png)
