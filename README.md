# mysqlCDC

```
### UPDATE `test`.`user`
### WHERE
###   @1=1
###   @2='5996b891-9d3c-4038-af37-cb07f5f0f72d'
###   @3=1
###   @4='Fulano da Silva Sauro'
###   @5='Sauro'
###   @6='sauro@pangea.com'
###   @7='admin'
### SET
###   @1=1
###   @2='5996b891-9d3c-4038-af37-cb07f5f0f72d'
###   @3=1
###   @4='name changed'
###   @5='Sauro'
###   @6='sauro@pangea.com'
###   @7='admin'
```

```
mysqlbinlog -v --base64-output=DECODE-ROWS /var/log/mysql-bin.000003
```


```
/bin/ash
/bin/sh
/ash
/sh
```