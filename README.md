# crawlers

In order to fetch and import the internal dependencies, please run this command:
```bash
export GOPRIVATE="github.com/duc-thien-phong,github.com/duc-thien-phong-internal"
git config --global url."git@github.com:".insteadOf "https://github.com/"
```

# Start CICB
```bash
./scripts/cicb/build.sh --os darwin && ./bin/executable/cicb/darwin/cicb
```