protoc \
      --plugin protoc-gen-go-lite="${GOBIN}/protoc-gen-go-lite" \
      --go-lite_out=./  \
      --go-lite_opt=features=marshal+unmarshal+size \
      ./casemod.proto
      
