// Dragon Market — black-box QA simulation.
//
// Drives a *running* server through realistic, often-adversarial scenarios and
// asserts the three guarantees hold: no duplicate/invalid sale, no
// over-commitment, and traceable/defensible behavior under concurrency.
//
// Usage:  BASE=http://localhost:8095 node scripts/qa.mjs   (or `make qa`)
// Assumes the demo seed (guilds ironband/stormforge/shadowveil) on a FRESH
// database, and a short AUCTION_WINDOW (e.g. 5s) so the settlement scenarios
// complete within the poll timeout. Start such a server with, for example:
//   AUCTION_WINDOW=5s AUCTION_EXTENSION=2s HTTP_ADDR=:8095 ./server

const BASE = process.env.BASE || "http://localhost:8095";

let pass = 0, fail = 0;
const failures = [];

function ok(name, cond, detail = "") {
  if (cond) { pass++; console.log(`  \x1b[32m✓\x1b[0m ${name}`); }
  else { fail++; failures.push(name + (detail ? ` — ${detail}` : "")); console.log(`  \x1b[31m✗ ${name}\x1b[0m ${detail}`); }
}
function section(t) { console.log(`\n\x1b[1m\x1b[35m${t}\x1b[0m`); }
const sleep = ms => new Promise(r => setTimeout(r, ms));

// api returns { status, body, headers }
async function api(method, path, { guild, body, key } = {}) {
  const headers = {};
  if (guild) headers["X-Guild-Id"] = guild;
  if (key) headers["Idempotency-Key"] = key;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const res = await fetch(BASE + path, { method, headers, body: body !== undefined ? JSON.stringify(body) : undefined });
  const text = await res.text();
  let parsed = null; try { parsed = text ? JSON.parse(text) : null; } catch {}
  return { status: res.status, body: parsed, headers: res.headers };
}

const wallet = g => api("GET", `/guilds/${g}/wallet`).then(r => r.body);
const items = () => api("GET", "/items").then(r => r.body);
const findItem = async name => (await items()).find(i => i.name === name);
const listItem = (seller, it) => api("POST", "/items", { guild: seller, body: it }).then(r => r.body);
const itemStatus = async id => (await api("GET", `/items/${id}`)).body?.status;

// waitForStatus polls until an item reaches `want` or the timeout elapses —
// robust to the settlement worker's tick. Requires a short AUCTION_WINDOW.
async function waitForStatus(id, want, timeoutMs = 25000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await itemStatus(id) === want) return true;
    await sleep(500);
  }
  return false;
}

async function main() {
  console.log(`\x1b[1mDragon Market QA\x1b[0m → ${BASE}\n`);

  section("1. Health & seed");
  ok("healthz 200", (await api("GET", "/healthz")).status === 200);
  const seeded = await items();
  ok("3 seeded items", seeded.length === 3, `got ${seeded.length}`);
  ok("a legendary item was seeded", !!seeded.find(i => i.rarity === "legendary"));
  ok("items expose advisory oracle price", seeded.every(i => typeof i.oracle_price === "number"));

  section("2. Wallet invariant baseline");
  const guilds = ["ironband", "stormforge", "shadowveil"];
  const startTotals = Object.fromEntries(await Promise.all(guilds.map(async g => [g, (await wallet(g)).total])));
  const startSum = Object.values(startTotals).reduce((a, b) => a + b, 0);
  ok("starting gold sum = 25000", startSum === 25000, `got ${startSum}`);

  section("3. Limit order — happy path");
  const potion = await findItem("Health Potion"); // ironband sells @50
  const before = (await wallet("stormforge")).total;
  const buy = await api("POST", `/items/${potion.id}/purchase`, { guild: "stormforge", body: { quantity: 2 } });
  ok("buy 2 potions → 200", buy.status === 200, `status ${buy.status}`);
  ok("cost = 100", buy.body?.cost === 100, `got ${buy.body?.cost}`);
  ok("stock decremented to 18", (await findItem("Health Potion")).quantity === 18);
  ok("buyer charged 100", (await wallet("stormforge")).total === before - 100);
  ok("seller credited 100", (await wallet("ironband")).total === startTotals.ironband + 100);

  section("4. Limit order — rejections");
  const own = await api("POST", `/items/${potion.id}/purchase`, { guild: "ironband", body: { quantity: 1 } });
  ok("seller cannot buy own item → 409", own.status === 409, `status ${own.status}`);
  ok("error mentions own item", /own item/i.test(own.body?.error || ""));
  const leg = seeded.find(i => i.rarity === "legendary");
  const legBuy = await api("POST", `/items/${leg.id}/purchase`, { guild: "ironband", body: { quantity: 1 } });
  ok("legendary not buyable via limit order → 409", legBuy.status === 409, `status ${legBuy.status}`);

  section("5. Daily purchase cap (shadowveil cap=2000)");
  const shield = await findItem("Mithril Shield"); // stormforge sells @800
  const b1 = await api("POST", `/items/${shield.id}/purchase`, { guild: "shadowveil", body: { quantity: 2 } });
  ok("buy 2 shields (1600) → 200", b1.status === 200, `status ${b1.status}`);
  const b2 = await api("POST", `/items/${shield.id}/purchase`, { guild: "shadowveil", body: { quantity: 1 } });
  ok("3rd shield would hit 2400 > cap → 409", b2.status === 409, `status ${b2.status}`);
  ok("error mentions cap", /cap/i.test(b2.body?.error || ""), b2.body?.error);

  section("6. Idempotency (replayed POST applies once)");
  const idemBefore = (await wallet("stormforge")).total;
  const k = "qa-key-" + Date.now();
  const r1 = await api("POST", `/items/${potion.id}/purchase`, { guild: "stormforge", body: { quantity: 1 }, key: k });
  const r2 = await api("POST", `/items/${potion.id}/purchase`, { guild: "stormforge", body: { quantity: 1 }, key: k });
  ok("first keyed buy → 200", r1.status === 200);
  ok("replay flagged Idempotent-Replayed", r2.headers.get("Idempotent-Replayed") === "true");
  ok("charged only once (50)", (await wallet("stormforge")).total === idemBefore - 50);

  section("7. Concurrency — single unit, many racers (no oversell)");
  const relic = await listItem("ironband", { name: "OneOfAKind", rarity: "rare", price: 100, quantity: 1 });
  const racers = Array.from({ length: 12 }, () =>
    api("POST", `/items/${relic.id}/purchase`, { guild: "stormforge", body: { quantity: 1 } }));
  const results = await Promise.all(racers);
  const wins = results.filter(r => r.status === 200).length;
  ok("exactly one racer wins", wins === 1, `wins=${wins}`);
  ok("item sold out", (await api("GET", `/items/${relic.id}`)).body.status === "sold_out");

  // One self-contained auction exercises bidding, reservation hand-off, cancel,
  // and settlement — using uncapped bidders (ironband/stormforge) so the daily
  // cap doesn't interfere, and waiting out its own settlement so it never
  // perturbs later balances.
  section("8. Auction — bidding, cancel, and settlement (self-contained)");
  const sword = await listItem("shadowveil", { name: "Frostbite", rarity: "legendary", price: 500 });
  ok("listing legendary opens auction (in_auction)", sword.status === "in_auction");
  const ironStart = (await wallet("ironband")).total, sellerStart = (await wallet("shadowveil")).total;

  ok("stormforge bid 500 → 201", (await api("POST", `/items/${sword.id}/bid`, { guild: "stormforge", body: { amount: 500 } })).status === 201);
  ok("stormforge reserves 500", (await wallet("stormforge")).reserved === 500);
  ok("ironband bid 510 (<+5%) → 409", (await api("POST", `/items/${sword.id}/bid`, { guild: "ironband", body: { amount: 510 } })).status === 409);
  ok("ironband bid 525 (+5%) → 201", (await api("POST", `/items/${sword.id}/bid`, { guild: "ironband", body: { amount: 525 } })).status === 201);
  ok("outbid stormforge reservation released", (await wallet("stormforge")).reserved === 0);
  ok("ironband now reserves 525", (await wallet("ironband")).reserved === 525);
  ok("seller cannot bid own auction → 409", (await api("POST", `/items/${sword.id}/bid`, { guild: "shadowveil", body: { amount: 600 } })).status === 409);

  const au = (await api("GET", `/auctions/${(await api("GET", "/auctions")).body.find(a => a.item_id === sword.id).id}`)).body;
  const stormBid = au.bids.find(b => b.guild_id === "stormforge");
  ok("highest bidder cannot cancel → 409", (await api("DELETE", `/items/${sword.id}/bid/${au.highest_bid.id}`, { guild: "ironband" })).status === 409);
  ok("non-leader can cancel own bid → 200", (await api("DELETE", `/items/${sword.id}/bid/${stormBid.id}`, { guild: "stormforge" })).status === 200);
  ok("cannot cancel another guild's bid → 4xx", (await api("DELETE", `/items/${sword.id}/bid/${au.highest_bid.id}`, { guild: "stormforge" })).status >= 400);

  process.stdout.write("  …waiting for window + worker to settle\n");
  ok("won legendary is sold_out", await waitForStatus(sword.id, "sold_out"));
  ok("winner (ironband) debited 525", (await wallet("ironband")).total === ironStart - 525);
  ok("winner reservation cleared", (await wallet("ironband")).reserved === 0);
  ok("seller (shadowveil) credited 525", (await wallet("shadowveil")).total === sellerStart + 525);
  const ledger = (await api("GET", "/guilds/ironband/ledger")).body;
  ok("ledger records reserve then settle", ledger.some(e => e.type === "reserve") && ledger.some(e => e.type === "settle" && e.amount === 525));

  section("9. No-bid auction returns item to available");
  const lonely = await listItem("ironband", { name: "LonelyBlade", rarity: "legendary", price: 999 });
  ok("expired no-bid auction → item available again", await waitForStatus(lonely.id, "available"));

  section("10. Daily cap also bounds auction bids");
  // shadowveil already spent 1600 of its 2000 cap on shields; a 500 bid would
  // realize 2100 > 2000 if it won, so it is rejected at bid time.
  const capItem = await listItem("ironband", { name: "CapProbe", rarity: "legendary", price: 500 });
  const capBid = await api("POST", `/items/${capItem.id}/bid`, { guild: "shadowveil", body: { amount: 500 } });
  ok("over-cap bid rejected → 409", capBid.status === 409, `status ${capBid.status}`);
  ok("error mentions cap", /cap/i.test(capBid.body?.error || ""), capBid.body?.error);

  section("11. Gold conservation invariant");
  const endTotals = Object.fromEntries(await Promise.all(guilds.map(async g => [g, (await wallet(g)).total])));
  const endSum = Object.values(endTotals).reduce((a, b) => a + b, 0);
  ok("total gold conserved across all trades (25000)", endSum === 25000, `got ${endSum}`);

  console.log(`\n\x1b[1mResult: \x1b[32m${pass} passed\x1b[0m, ${fail ? `\x1b[31m${fail} failed\x1b[0m` : "0 failed"}\x1b[0m`);
  if (fail) { console.log("Failures:\n - " + failures.join("\n - ")); process.exit(1); }
}

main().catch(e => { console.error("QA crashed:", e); process.exit(2); });
