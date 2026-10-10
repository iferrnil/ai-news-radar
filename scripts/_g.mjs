import { chromium } from "playwright";
const b = await chromium.launch(); const pg = await b.newPage();
await pg.goto("http://localhost:3000/",{waitUntil:"networkidle"}); await pg.waitForTimeout(1500);
console.log(await pg.evaluate(()=>[...new Set([...document.fonts].filter(f=>f.status==="loaded").map(f=>f.family+" "+f.weight))].join(" | ")));
await b.close();
