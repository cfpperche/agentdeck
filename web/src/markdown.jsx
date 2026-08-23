import { useEffect, useRef } from "react";
import { marked } from "marked";
import DOMPurify from "dompurify";
import hljs from "highlight.js/lib/common";

marked.setOptions({ breaks: true, gfm: true });

export function Markdown({ text }) {
  const ref = useRef(null);
  useEffect(() => {
    if (!ref.current) return;
    ref.current.querySelectorAll("pre code").forEach((el) => {
      try { hljs.highlightElement(el); } catch {}
      const pre = el.parentElement;
      if (!pre.querySelector(".copy-btn")) {
        const btn = document.createElement("button");
        btn.className = "copy-btn";
        btn.textContent = "copiar";
        btn.onclick = () => {
          navigator.clipboard.writeText(el.textContent);
          btn.textContent = "✓ copiado";
          setTimeout(() => (btn.textContent = "copiar"), 1200);
        };
        pre.appendChild(btn);
      }
    });
  }, [text]);
  const html = DOMPurify.sanitize(marked.parse(text || ""));
  return <div ref={ref} class="md" dangerouslySetInnerHTML={{ __html: html }} />;
}
