- t:<txid>
HASH
r - rawtx
p - merkleproof
b - block


- o<outpoint>
HASH
txo - txo
spend - spend

    
owner:bsv21:id:<id>
ZSET - outpoint, dx


evt:<tag>:<id>:<value>
outpoint -> score

oev:<owner>:<tag>:<id>:<value>
outpoint -> score

own:<owner>
outpoint -> score


tx:txid
    a
    i%08x
    o%08x
    t