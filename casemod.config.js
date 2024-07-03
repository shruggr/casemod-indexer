const fs = require('fs');

const env = {
    JUNGLEBUS: "https://junglebus.gorillapool.io",
    REDISDB: "127.0.0.1:6379",
    REDISCACHE: "127.0.0.1:6379",
    ARC: "https://arc.gorillapool.io",
}

// const bitcoinConf = fs.readFileSync('./.bitcoin/bitcoin.conf');
// bitcoinConf.toString().split('\n').forEach(line => {
//     const parts = line.split('=');
//     switch(parts[0]) {
//         case 'rpcconnect':
//             env.BITCOIN_HOST = parts[1];
//             break;
//         case 'rpcport':
//             env.BITCOIN_PORT = parts[1];
//             break;
//         case 'rpcuser':
//             env.BITCOIN_USER = parts[1];
//             break;
//         case 'rpcpassword':
//             env.BITCOIN_PASS = parts[1];
//             break;
//     }
// })
module.exports = {
    apps: [
        {
            name: "listener",
            script: "cmd/listener/listener",
            args: "-id=bsv21 -t=22826aa9edbd03832bd1024866dab85d6abeade94eb011e5a3c3a59f5abdbe26 -s=811302 -v=0",
            env,
        }, {
            name: "index",
            script: "cmd/bsv21/bsv21",
            args: "",
            env,
        }    
    ]
}

