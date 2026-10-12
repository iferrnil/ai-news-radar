#!/usr/bin/env bash
# Input files for the import tests (t2/t3), written to /tmp. subset.json needs t1's /tmp/export-all.json; runall.sh calls this before each test.
cat > /tmp/ext.opml <<'X'
<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0"><head><title>Feedly export</title></head><body>
<outline text="Tech" title="Tech">
  <outline type="rss" text="Hacker News" xmlUrl="https://hnrss.org/frontpage" htmlUrl="https://news.ycombinator.com/"/>
  <outline type="rss" text="The Verge AI" xmlUrl="https://www.theverge.com/rss/ai-artificial-intelligence/index.xml"/>
  <outline type="rss" text="Techmeme copy" xmlUrl="http://techmeme.com/feed.xml/"/>
  <outline type="rss" text="Bad FTP" xmlUrl="ftp://example.com/feed.xml"/>
</outline>
<outline text="国内媒体" title="国内媒体">
  <outline type="rss" text="IT之家" xmlUrl="https://www.ithome.com/rss/"/>
</outline>
<outline type="rss" text="Simon Willison" category="实验室" xmlUrl="https://simonwillison.net/atom/everything/"/>
</body></opml>
X
node -e '
const fs=require("fs");const H={app:"ai-news-radar",version:1};
fs.writeFileSync("/tmp/bad-urls.json",JSON.stringify({...H,sources:[{name:"JS",type:"rss",url:"javascript:alert(1)"},{name:"File",type:"rss",url:"file:///etc/passwd"},{name:"OK-ish",type:"rss",url:"https://example.org/feed"}]}));
fs.writeFileSync("/tmp/malformed.json","{\"app\":\"ai-news-radar\", \"version\":1, sources:[");
fs.writeFileSync("/tmp/big.json",JSON.stringify({...H,sources:[],pad:"x".repeat(1100000)}));
fs.writeFileSync("/tmp/update.json",JSON.stringify({...H,sources:[{name:"OpenAI News",type:"rss",url:"https://openai.com/news/rss.xml",section:"media",priority:"P2",interval:1800,enabled:false}]}));
if(fs.existsSync("/tmp/export-all.json")){const all=JSON.parse(fs.readFileSync("/tmp/export-all.json","utf8"));
fs.writeFileSync("/tmp/subset.json",JSON.stringify({...H,sources:all.sources.slice(0,3)}));}
'
