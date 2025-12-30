### listcurrentcrs

Show current cr members information

#### Parameter
| name  | type    | description                                                  |
| ----- | ------- | ------------------------------------------------------------ |
| state | string  | the cr member state you want know <br/>

#### Result
| name            | type   | description                           |
| --------------  | ------ |---------------------------------------|
| code            | string | the cr member code                    |
| cid             | string | the cr member address                 |
| did             | string | the cr member did address             |
| dpospublickey   | string | the cr member dpos public key         |
| nickname        | string | the nick name of the cr member        |
| url             | string | the url of the cr member              |
| location        | uint64 | the location number of the cr member  |
| impeachmentvotes| string  | impeachment votes of the cr member   |
| depositamount   | string | the deposite amout of the cr member   |
| depositaddress  | string | the deposite address of the cr member |
| penalty         | string  | the penalty of the cr member         |
| index           | uint64 | the index of the cr member            |
| totalcounts     | uint64 | the total counts of current cr member |


#### Example

Request:
```json
{
"method": "listcurrentcrs",
  "params":{
    "state":"all"
  }
}
```

Response:
```json
{
    "error": null,
    "id": null,
    "jsonrpc": "2.0",
    "result": {
        "crmembersinfo": [
            {
                "code": "2102e23f70b9b967af35571c32b1442d787c180753bbed5cd6e7d5a5cfe75c7fc1ffac",
                "cid": "iaiZJM922uWo2Uc2gYwZk1nEgiVV7NTtxR",
                "did": "inTc9GeWyNNKNwT1cDcvvEgQwnjszbtpZ5",
                "dpospublickey": "02a91b4ccc0204e2964719fe5035c06af1d5141968eb1e5117f74feee625eac4d5",
                "nickname": "ela_cr2",
                "url": "ela_cr2.org",
                "location": 112211, 
                "impeachmentvotes": "0.2",
                "depositamout": "5000",
                "deposithash": "De87Qiekzpx7Xqf8RphdwNX5Z84iGgHLKMF5b",
                "penalty": "1.1",
                "index": 0,
                "State": "Elected"
            },
            {
                "code": "2103c3dd01baa4e3d0625f6c0026ad3d06d085e80c57477efa1a4aa2ab209c210e95ac",
                "cid": "iUBoqE5KnBA1zsd4EWeyj2mXMfUrm5rDmf",
                "did": "intySungjAK3uyHeoajez3yRqX5x68NrNi",
                "dpospublickey": "02b07c2a60de0b42e8fb11a69976f8e1fac29024786a4d36aa26fcfc1fca52fd9b",
                "nickname": "ela_cr1",
                "url": "ela_cr1.org",
                "location": 112211,
                "impeachmentvotes": "0.1",
                "depositamout": "5000",
                "depositaddress": "DnemZpPgHLKMF5bMX3WbJYSGTpqJkBN7pe",
                "penalty": "0.2",
                "index": 1,
                "State": "Elected"
            }
        ],
        "totalcounts": 2
    }
}
```


### listproducers

Show producers infromation

#### Parameter 

| name     | type    | description                                                                                                               |
|----------|---------|---------------------------------------------------------------------------------------------------------------------------|
| start    | integer | optional, the start index of producers                                                                                    |
| limit    | integer | optional, the limit count of producers                                                                                    |
| identity | string  | optional, the identity of the producers, the default value is `all`, and the valid input is `v1`, `v2` and `all`          |
| state    | string  | the producer state you want<br/>"all": get producers in any state<br/>"pending": get producers in the pendding state<br/> 
"active": get producers in the active state<br/>
"inactive": get producers in the inactive state<br/>
"canceled": get producers in the canceled state<br/>
"illegal": get producers in the illegal state<br/>
"returned": get producers in the returned state |
if state flag not provided return the producers in pending and active state.

#### Result

| name             | type   | description                                                    |
|------------------| ------ |----------------------------------------------------------------|
| ownerpublickey   | string | the owner public key of producer                               |
| nodepublickey    | string | the node public key of the producer                            |
| nickname         | string | the nick name of the producer                                  |
| url              | string | the url of the producer                                        |
| location         | uint64 | the location number of the producer                            |
| active           | bool   | if producer has confirmed                                      |
| votes            | string | the votes currently held                                       |
| state            | string | the current state of the producer                              |
| onduty           | string | the current on duty state of the DPoS 2.0 node                 |
| identity         | string | the current identity of the DPoS node                          |
| registerheight   | uint32 | the height of cancel producer                                  |
| cancelheight     | uint32 | the cancel height of the producer                              |
| inactiveheight   | uint32 | the inactive start height of the producer                      |
| illegalheight    | uint32 | the illegal start height of the producer                       |
| index            | uint64 | the index of the producer                                      |
| totalvotes       | string | the total votes of the DPoS 1.0 nodes(will be deprecated soon) |
| totaldposv1votes | string | the total votes of the DPoS 1.0 nodes                          |
| totaldposv2votes | string | the total votes of the DPoS 2.0 nodes                          |
| totalcounts      | uint64 | the total counts of registered producers                       |

#### Example

Request:

```json
{
  "method": "listproducers",
  "params":{
    "start": 0,
    "limit": 3,
    "identity":"V2"
  }
}
```

Response:

```json
{
  "error": null,
  "id": null,
  "jsonrpc": "2.0",
  "result": {
    "producers": [
      {
        "ownerpublickey": "0237a5fb316caf7587e052125585b135361be533d74b5a094a68c64c47ccd1e1eb",
        "nodepublickey": "0237a5fb316caf7587e052125585b135361be533d74b5a094a68c64c47ccd1e1eb",
        "nickname": "elastos1",
        "url": "http://www.elastos1.com",
        "location": 401,
        "active": true,
        "votes": "3.11100000",
        "dposv2votes": "3.11100000",
        "state": "Active",
        "onduty": "valid",
        "identity": "DPoSV1V2",
        "registerheight": 236,
        "cancelheight": 0,
        "inactiveheight": 0,
        "illegalheight": 0,
        "index": 0
      },
      {
        "ownerpublickey": "030a26f8b4ab0ea219eb461d1e454ce5f0bd0d289a6a64ffc0743dab7bd5be0be9",
        "nodepublickey": "030a26f8b4ab0ea219eb461d1e454ce5f0bd0d289a6a64ffc0743dab7bd5be0be9",
        "nickname": "elastos2",
        "url": "http://www.elastos2.com",
        "location": 402,
        "active": true,
        "votes": "2.10000000",
        "dposv2votes": "2.10000000",
        "state": "Active",
        "onduty": "valid",
        "identity": "DPoSV1V2",
        "registerheight": 225,
        "cancelheight": 0,
        "inactiveheight": 0,
        "illegalheight": 0,
        "index": 1
      },
      {
        "ownerpublickey": "0288e79636e41edce04d4fa95d8f62fed73a76164f8631ccc42f5425f960e4a0c7",
        "nodepublickey": "0288e79636e41edce04d4fa95d8f62fed73a76164f8631ccc42f5425f960e4a0c7",
        "nickname": "elastos3",
        "url": "http://www.elastos3.com",
        "location": 403,
        "active": true,
        "votes": "0",
        "dposv2votes": "0", 
        "state": "Active",
        "onduty": "invalid",
        "identity": "DPoSV1V2",
        "registerheight": 216,
        "cancelheight": 0,
        "inactiveheight": 0,
        "illegalheight": 0,
        "index": 2
      }
    ],
    "totalvotes": "5.21100000",
    "totaldposv1votes": "5.21100000",
    "totaldposv2votes": "5.21100000",
    "totalcounts": 10
  }
}
```
