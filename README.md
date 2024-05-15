# gosyntect

gosyntect is now part of the [Sourcegraph monorepo](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:gosyntect&patternType=keyword&sm=0).

The information below is preserved for archival purposes only.

gosyntect is a Go HTTP client for [syntect_server](https://github.com/sourcegraph/syntect_server), a Rust HTTP server which syntax highlights code.

## Installation

```Bash
go get -u github.com/sourcegraph/gosyntect/cmd/gosyntect
```

## Usage

```
usage: gosyntect <server> <theme> <file.go>

example:
	gosyntect http://localhost:9238 'InspiredGitHub' gosyntect.go
```

## API

```Go
client := gosyntect.New("http://localhost:9238")
resp, err := cl.Highlight(&gosyntect.Query{
	Extension: "go",
	Theme:     "InspiredGitHub",
	Code:      string(theGoCode),
})
if err != nil {
	log.Fatal(err)
}
fmt.Println(resp.Data) // prints highlighted HTML
```
