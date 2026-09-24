# Memories: the name, and the story

This project started life as **GC** — *Golang Coin*.

I named it GC because I love Golang. I love how the language feels, and I
love its mascot: the gopher. Since this was my take on building a coin —
a mini Bitcoin implementation I wrote by reading Bitcoin's source and
figuring out Go on the fly — "GC" just made sense: **G**o + **C**oin.

Two years later, the node grew up. UTXO set, mempool, peer eviction,
difficulty retargeting, a signing-hash scheme, satoshis that are actually
satoshis. It deserved a name that sounds like a name, not an
abbreviation. So:

**GC → GoCoNakamoto** — the Go coin, with Satoshi in the family tree.

---

## The backstory

I don't buy stocks or shares in other companies. It feels like a
betrayal — why would I put money on someone else's company? Why
wouldn't I use that money on my own stuff? (Well, partly because I
couldn't afford it anyway — but it's a principle. I hated the idea.)
But I was fascinated by Bitcoin. And since I didn't know C++ or
anything else, I decided to reverse engineer the code, one step at a
time.

There was ChatGPT back then; no Claude that I was aware of (I honestly
don't even know when Claude launched). Funny backstory: I asked GPT
"if you had to write BTC, how would you write it?" — and chatbots were
pretty stupid at the time, so it wrote boilerplate. I looked at it,
having never seen Bitcoin ever, and thought: *whatever Bitcoin is, this
cannot be it.* I refused to believe it. So I asked it to find me the
repo — and thankfully, it did.

I was new to computers and programming, so I was kind of lost all the
time. I had just gotten my first ever Mac, which my sister bought for
me from her two years of savings. I'm eternally grateful for that. I
don't care for people betting on me or vouching for me — I'm not
neurotypical, I'm not bothered by such things — but I couldn't help but
be grateful, because we aren't rich, or even middle class. It took a
while before we could afford that Mac.

Once I had the link to the source code, I started reading and did not
understand any of it. I sat there for days. No progress. Everything
was a giant mess. So I asked GPT to tell me the most important file in
that codebase — the one that does all the cool stuff. It pointed me to
a file (I can't recall the name — some `.h`/`.c`/`.cpp` set, one of
them ~5,000 lines). I didn't get any of it for almost two months of
staring at the code. Slowly I would copy the code over, hand it to GPT
to translate — I knew a little JavaScript, very little — and I'd assume
reasons for why things were being called. Eventually I started piecing
things together. And once I *got* it, I accidentally found the source
code for v1 of Bitcoin — and now that I understood it, looking at v1
was trivial. That gave me enough to go build my own. And here we are.

## The seeder, the autoscaler, and the magic going away

As I started building, I realized we'd need a seeder — something to
distribute who's up, and what kind of node they run. I knew we coded a
CNode, a full node. But as soon as I built a single version, I realized
that if nodes rely on each other and we ignore hardcoded seed IPs/DNS,
there's a single point of failure: if the DNS seeder goes down, the
whole network is fucked. So I needed something that makes sure that
doesn't happen — and that's how I discovered what an autoscaler is.

Since I couldn't afford one and my goal was always *understanding how
these systems work*, I used the Docker CLI to build my own version of
an autoscaler: [Content_Delivery_Network](https://github.com/RahulMishra0722/Content_Delivery_Network).
We created an image of the seeder and ran multiple instances of the
node. That's how I learned about reverse proxies, leaky buckets,
sliding windows, and other rate-limiting algorithms.

And as soon as I had them working — doing exactly what they're supposed
to do — **the magic went away.** That's the pattern: magic is what
you haven't understood yet.

I had given myself a deadline before starting my company. Time was
running out, so as a last thing I made a neural net trained on MNIST
for image classification, and finally a transformer in C. That's all I
had time for. Then the startup journey began!

## Where this sits

Looking forward to using the learnings from my previous startup,
**Edison** — which is live and has happy customers. I realized it had a
cap (how much I could scale it in ARR) and I wasn't happy with it, so I
decided to pivot to **Adverdict**. I know for a fact: this is it.

This note exists so the old name is never a mystery — not even to me.
Renamed before going live with Adverdict, the business I'm building
with high hopes. If you're reading this from the future: it all started
with a gopher, a sister's savings, and a guy who dropped out of college
but never stopped loving this stuff.

— Rahul, 2026
