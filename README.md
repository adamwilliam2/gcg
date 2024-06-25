# gcg
gen code for gorm

# using private repo (gopay)

1. set go env
```
go env -w GOPRIVATE=github.com/SDLyu/gopay
```

2. replace the private package
```
replace gopay v1.0.0 => github.com/SDLyu/gopay v1.0.0
```

3. put your file under output folder

4. gen code via Makefile
```
make gen
```

