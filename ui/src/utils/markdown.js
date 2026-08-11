const escapeHtml = (value = '') =>
    value
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');

const safeUrl = (value = '') => {
    try {
        const url = new URL(value, window.location.origin);
        return ['http:', 'https:'].includes(url.protocol) ? url.href : '#';
    } catch (e) {
        return '#';
    }
};

const renderInline = (value = '') => {
    const links = [];
    const textWithLinkTokens = value.replace(
        /\[([^\]]+)]\(([^\s)]+)\)/g,
        (match, label, url) => {
            const token = `MARKDOWNLINKTOKEN${links.length}END`;
            links.push(
                `<a href="${escapeHtml(safeUrl(url))}" target="_blank" rel="noopener noreferrer">${escapeHtml(label)}</a>`,
            );
            return token;
        },
    );

    return escapeHtml(textWithLinkTokens)
        .replace(/`([^`]+)`/g, '<code>$1</code>')
        .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
        .replace(/__([^_]+)__/g, '<strong>$1</strong>')
        .replace(/(^|[^*])\*([^*]+)\*/g, '$1<em>$2</em>')
        .replace(/MARKDOWNLINKTOKEN(\d+)END/g, (match, index) => links[Number(index)] || '');
};

export const markdownToHtml = (markdown = '') => {
    const lines = String(markdown).replace(/\r\n?/g, '\n').split('\n');
    const output = [];
    let listType = '';
    let inCodeBlock = false;
    let codeLines = [];

    const closeList = () => {
        if (listType) {
            output.push(`</${listType}>`);
            listType = '';
        }
    };

    lines.forEach((line) => {
        if (/^```/.test(line)) {
            closeList();
            if (inCodeBlock) {
                output.push(`<pre><code>${escapeHtml(codeLines.join('\n'))}</code></pre>`);
                codeLines = [];
            }
            inCodeBlock = !inCodeBlock;
            return;
        }

        if (inCodeBlock) {
            codeLines.push(line);
            return;
        }

        const unordered = line.match(/^\s*[-*+]\s+(.+)$/);
        const ordered = line.match(/^\s*\d+[.)]\s+(.+)$/);
        if (unordered || ordered) {
            const nextType = unordered ? 'ul' : 'ol';
            if (listType !== nextType) {
                closeList();
                listType = nextType;
                output.push(`<${listType}>`);
            }
            output.push(`<li>${renderInline((unordered || ordered)[1])}</li>`);
            return;
        }

        closeList();
        if (!line.trim()) return;

        const heading = line.match(/^(#{1,6})\s+(.+)$/);
        if (heading) {
            const level = heading[1].length;
            output.push(`<h${level}>${renderInline(heading[2])}</h${level}>`);
            return;
        }

        const quote = line.match(/^>\s?(.+)$/);
        if (quote) {
            output.push(`<blockquote>${renderInline(quote[1])}</blockquote>`);
            return;
        }

        output.push(`<p>${renderInline(line)}</p>`);
    });

    closeList();
    if (inCodeBlock && codeLines.length) {
        output.push(`<pre><code>${escapeHtml(codeLines.join('\n'))}</code></pre>`);
    }

    return output.join('\n');
};
