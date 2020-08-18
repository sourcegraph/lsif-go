# Indexer performance

We ran lsif-go (v1.0.0) over repositories of various sizes to determine the performance characteristics of the indexer as a function of its input. The machine running this benchmark was a iMac Pro (2017) with a 2.3 GHz 8-Core Intel Xeon W and 64GB of RAM. Performance characteristics may differ with a process of a different speed or a different number of cores (especially as the repository size increases).

| Repo name   | Repo size | SLoC       | Comment LoC | Time to index | Index size |
| ----------- | --------- | ---------- | ----------- | ------------- | ---------- |
| monorepo-1  |      268M |  1,314,700 |   1,036,663 |    0m 23.808s |      1.4G  |
| monorepo-5  |      633M |  6,220,940 |   5,093,927 |    1m 47.697s |      6.9G  |
| monorepo-10 |      1.1G | 12,353,740 |  10,165,507 |    3m 34.579s |       13G  |
| monorepo-15 |      1.5G | 18,486,540 |  15,237,087 |    8m 12.479s |       20G  |
| monorepo-20 |      2.0G | 24,619,340 |  20,308,667 |   13m  0.855s |       27G  |
| monorepo-25 |      2.4G | 30,752,140 |  25,380,247 |   18m 52.822s |       33G  |

Notes: 
- SLOC = significant lines of code
- Comment LoC = number of comment lines
- Time to index is the average over 5 runs on an otherwise idle machine
- Index size is the size of the (uncompressed) output of the indexer

#### Source code generation

The source code used for indexing was generated from the following script. This will clone the Go AWS SDK (which is already a large-ish repository with many packages and large generated files with many symbols) and replicate the `services` directory a number of times to artificially expand the size of the repository.

```bash
#!/bin/bash -exu

N=${1:-5}
git clone git@github.com:aws/aws-sdk-go.git "monorepo-$N"
pushd "monorepo-$N"

mv service service1
find . -type f -name '*.go' -exec \
    sed -i '' \
    -e 's/github.com\/aws\/aws-sdk-go\/service/github.com\/aws\/aws-sdk-go\/service1/g' \
    {} \;

if [ 2 -lt "$N" ]; then
  for i in $(seq 2 "$N"); do
    cp -r service1 "service$i"
    pushd "service$i"
    find . -type f -name '*.go' -exec sed -i '' \
        -e "s/service1/service$i/g" \
        {} \;
    popd
  done
fi

popd
```

The benchmark results in this document used the commit `20cd465d`.
