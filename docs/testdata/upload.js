const fs = require("fs");
const path = require("path");

const BASE_URL = process.env.REVIEWSRV_URL || "http://localhost:8075";
const PROJECT_KEY = process.env.PROJECT_KEY;
const DIR = process.env.REVIEW_DIR || ".";

const TYPES = { R1: "architecture", R2: "code", R3: "security", R4: "tests" };

async function upload(url, body, contentType = "application/octet-stream") {
  const res = await fetch(url, { method: "POST", body, headers: { "Content-Type": contentType } });
  const text = await res.text();
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${text}`);
  }
  return text;
}

async function main() {
  const reviewJSON = fs.readFileSync(path.join(DIR, "review.json"));
  const reviewId = await upload(`${BASE_URL}/v1/upload/${PROJECT_KEY}/`, reviewJSON, "application/json");
  console.log(`reviewId=${reviewId}`);

  const files = fs.readdirSync(DIR);
  for (const [prefix, type] of Object.entries(TYPES)) {
    const file = files.find((f) => f.startsWith(prefix + ".") && f.endsWith(".md"));
    if (!file) continue;
    const content = fs.readFileSync(path.join(DIR, file));
    await upload(`${BASE_URL}/v1/upload/${PROJECT_KEY}/${reviewId}/${type}/`, content);
    console.log(`uploaded ${file}`);
  }
}

main().catch((e) => { console.error(e.message); process.exit(1); });
