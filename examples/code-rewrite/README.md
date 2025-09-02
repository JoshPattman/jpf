# Code Rewrite
- Rewrites a source file into a specified language.
- May take extra pointers.
- Makes a single LLM call.

## Usage
- Make sure to set environment variable OPENAI_KEY to your openai key.
- `$ go run . -h`: Help on all arguments.
- `$ go run . -i <input file> -o <output file> -l <language> -p <pointer1>;<pointer2>;...`: Run the code rewriting.