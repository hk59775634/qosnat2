#!/usr/bin/env python3
"""Apply SPEC-02 Dynamic Split Tunneling (DST) edits to ocserv 1.5.0.

Must run on a tree that already has SPEC-01 applied:
  python3 scripts/apply-spec01-edits.py ocserv-src
  python3 scripts/apply-dst-edits.py ocserv-src

Idempotent. Supports:
  - dynamic-split-include-domains
  - dynamic-split-exclude-domains
  - Enhanced (both) → single X-CSTP-Post-Auth-XML
"""
from __future__ import annotations

import sys
from pathlib import Path


def insert_before(text: str, needle: str, insert: str, label: str) -> str:
    idx = text.find(needle)
    if idx < 0:
        raise SystemExit(f"{label}: anchor not found")
    if insert.strip() in text:
        print(f"{label}: already present")
        return text
    return text[:idx] + insert + text[idx:]


def insert_after(text: str, needle: str, insert: str, label: str) -> str:
    idx = text.find(needle)
    if idx < 0:
        raise SystemExit(f"{label}: anchor not found")
    if insert.strip() in text:
        print(f"{label}: already present")
        return text
    pos = idx + len(needle)
    return text[:pos] + insert + text[pos:]


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if new.strip() in text and old not in text:
        print(f"{label}: already present")
        return text
    if old not in text:
        raise SystemExit(f"{label}: anchor not found")
    return text.replace(old, new, 1)


WORKER_DST_HELPERS = r'''
/* SPEC-02: Dynamic Split Tunneling helpers (AnyConnect Post-Auth XML). */
static int dst_domain_in_csv(const char *csv, const char *tok)
{
	const char *p = csv;

	if (csv == NULL || tok == NULL)
		return 0;
	while (*p) {
		const char *comma = strchr(p, ',');
		size_t len = comma ? (size_t)(comma - p) : strlen(p);

		if (len == strlen(tok) && strncmp(p, tok, len) == 0)
			return 1;
		if (comma == NULL)
			break;
		p = comma + 1;
	}
	return 0;
}

static char *dst_join_domains(void *pool, char **entries, unsigned int n)
{
	char *out = NULL;
	unsigned int i;

	for (i = 0; i < n; i++) {
		char *dup, *tok, *save = NULL;

		if (entries[i] == NULL || entries[i][0] == 0)
			continue;
		dup = talloc_strdup(pool, entries[i]);
		if (dup == NULL)
			return out;
		for (tok = strtok_r(dup, ",", &save); tok != NULL;
		     tok = strtok_r(NULL, ",", &save)) {
			char *end;

			while (*tok == ' ' || *tok == '\t')
				tok++;
			end = tok + strlen(tok);
			while (end > tok && (end[-1] == ' ' || end[-1] == '\t'))
				*--end = 0;
			if (*tok == 0)
				continue;
			if (out == NULL) {
				out = talloc_strdup(pool, tok);
			} else if (!dst_domain_in_csv(out, tok)) {
				out = talloc_asprintf(pool, "%s,%s", out, tok);
			}
		}
		talloc_free(dup);
	}
	return out;
}

static char *dst_custom_attr_xml(void *pool, const char *include_csv,
				 const char *exclude_csv)
{
	char *inc = NULL, *exc = NULL, *attr;

	if (include_csv && include_csv[0])
		inc = talloc_asprintf(
			pool,
			"<dynamic-split-include-domains><![CDATA[%s]]></dynamic-split-include-domains>",
			include_csv);
	if (exclude_csv && exclude_csv[0])
		exc = talloc_asprintf(
			pool,
			"<dynamic-split-exclude-domains><![CDATA[%s]]></dynamic-split-exclude-domains>",
			exclude_csv);
	if (inc == NULL && exc == NULL)
		return NULL;
	attr = talloc_asprintf(pool, "%s%s", inc ? inc : "", exc ? exc : "");
	talloc_free(inc);
	talloc_free(exc);
	return attr;
}

static char *dst_build_post_auth_xml(void *pool, const char *include_csv,
				     const char *exclude_csv)
{
	char *attr = dst_custom_attr_xml(pool, include_csv, exclude_csv);
	char *xml;

	if (attr == NULL)
		return NULL;
	xml = talloc_asprintf(
		pool,
		"<?xml version=\"1.0\" encoding=\"UTF-8\"?>"
		"<config-auth client=\"vpn\" type=\"complete\" aggregate-auth-version=\"2\">"
		"<config client=\"vpn\" type=\"private\">"
		"<opaque is-for=\"vpn-client\">"
		"<custom-attr>%s</custom-attr>"
		"</opaque>"
		"</config>"
		"</config-auth>",
		attr);
	talloc_free(attr);
	return xml;
}

static char *dst_replace_cdata_tag(void *pool, char *body, const char *tag,
				   const char *csv)
{
	char open_tag[128];
	char *p, *end, *out;

	if (csv == NULL || csv[0] == 0)
		return body;
	snprintf(open_tag, sizeof(open_tag), "<%s>", tag);
	p = strcasestr(body, open_tag);
	if (p == NULL)
		return body;
	p = strstr(p, "<![CDATA[");
	if (p == NULL)
		return body;
	p += sizeof("<![CDATA[") - 1;
	end = strstr(p, "]]>");
	if (end == NULL)
		return body;
	out = talloc_asprintf(pool, "%.*s%s%s", (int)(p - body), body, csv, end);
	talloc_free(body);
	return out ? out : body;
}

static char *dst_ensure_tag(void *pool, char *body, const char *tag,
			    const char *csv)
{
	char open_tag[128];
	char *p, *insert, *out;

	if (csv == NULL || csv[0] == 0)
		return body;
	snprintf(open_tag, sizeof(open_tag), "<%s>", tag);
	if (strcasestr(body, open_tag) != NULL)
		return dst_replace_cdata_tag(pool, body, tag, csv);

	insert = talloc_asprintf(pool, "<%s><![CDATA[%s]]></%s>", tag, csv, tag);
	if (insert == NULL)
		return body;
	p = strcasestr(body, "</custom-attr>");
	if (p != NULL) {
		out = talloc_asprintf(pool, "%.*s%s%s", (int)(p - body), body,
				     insert, p);
		talloc_free(insert);
		talloc_free(body);
		return out ? out : body;
	}
	p = strcasestr(body, "</opaque>");
	if (p != NULL) {
		out = talloc_asprintf(pool,
				     "%.*s<custom-attr>%s</custom-attr>%s",
				     (int)(p - body), body, insert, p);
		talloc_free(insert);
		talloc_free(body);
		return out ? out : body;
	}
	talloc_free(insert);
	return body;
}

/* Merge include/exclude domains into an existing X-CSTP-Post-Auth-XML header. */
static char *dst_merge_post_auth_header(void *pool, const char *hdr,
					const char *include_csv,
					const char *exclude_csv)
{
	const char *prefix = "X-CSTP-Post-Auth-XML:";
	const char *xml;
	char *body;
	size_t prefix_len = sizeof("X-CSTP-Post-Auth-XML:") - 1;

	if (hdr == NULL)
		return NULL;
	if ((include_csv == NULL || include_csv[0] == 0) &&
	    (exclude_csv == NULL || exclude_csv[0] == 0))
		return NULL;
	if (strncasecmp(hdr, prefix, prefix_len) != 0)
		return NULL;
	xml = hdr + prefix_len;
	while (*xml == ' ' || *xml == '\t')
		xml++;
	body = talloc_strdup(pool, xml);
	if (body == NULL)
		return NULL;

	body = dst_ensure_tag(pool, body, "dynamic-split-include-domains",
			      include_csv);
	body = dst_ensure_tag(pool, body, "dynamic-split-exclude-domains",
			      exclude_csv);
	if (body == NULL)
		return NULL;
	return talloc_asprintf(pool, "X-CSTP-Post-Auth-XML: %s", body);
}

'''

EMIT_BLOCK = (
    "\t{\n"
    "\t\tchar *dst_include = NULL;\n"
    "\t\tchar *dst_exclude = NULL;\n"
    "\t\tunsigned int dst_sent = 0;\n"
    "\n"
    "\t\tif (ws->user_config->n_dynamic_split_include_domains > 0) {\n"
    "\t\t\tdst_include = dst_join_domains(\n"
    "\t\t\t\tws, ws->user_config->dynamic_split_include_domains,\n"
    "\t\t\t\tws->user_config->n_dynamic_split_include_domains);\n"
    "\t\t\tif (dst_include)\n"
    "\t\t\t\toclog(ws, LOG_DEBUG,\n"
    '\t\t\t\t      "dynamic-split-include-domains: %s", dst_include);\n'
    "\t\t}\n"
    "\t\tif (ws->user_config->n_dynamic_split_exclude_domains > 0) {\n"
    "\t\t\tdst_exclude = dst_join_domains(\n"
    "\t\t\t\tws, ws->user_config->dynamic_split_exclude_domains,\n"
    "\t\t\t\tws->user_config->n_dynamic_split_exclude_domains);\n"
    "\t\t\tif (dst_exclude)\n"
    "\t\t\t\toclog(ws, LOG_DEBUG,\n"
    '\t\t\t\t      "dynamic-split-exclude-domains: %s", dst_exclude);\n'
    "\t\t}\n"
    "\n"
    "\t\tfor (i = 0; i < WSRCONFIG(ws)->n_custom_header; i++) {\n"
    "\t\t\tchar *h = replace_vals(ws, WSRCONFIG(ws)->custom_header[i]);\n"
    "\n"
    "\t\t\tif (h) {\n"
    "\t\t\t\tif ((dst_include || dst_exclude) &&\n"
    '\t\t\t\t    strncasecmp(h, "X-CSTP-Post-Auth-XML:", 21) == 0) {\n'
    "\t\t\t\t\tchar *merged = dst_merge_post_auth_header(\n"
    "\t\t\t\t\t\tws, h, dst_include, dst_exclude);\n"
    "\n"
    "\t\t\t\t\tif (merged != NULL) {\n"
    "\t\t\t\t\t\ttalloc_free(h);\n"
    "\t\t\t\t\t\th = merged;\n"
    "\t\t\t\t\t\tdst_sent = 1;\n"
    "\t\t\t\t\t}\n"
    "\t\t\t\t}\n"
    '\t\t\t\toclog(ws, LOG_INFO, "adding custom header \'%s\'", h);\n'
    '\t\t\t\tret = cstp_printf(ws, "%s\\r\\n", h);\n'
    "\t\t\t\tSEND_ERR(ret);\n"
    "\t\t\t\ttalloc_free(h);\n"
    "\t\t\t}\n"
    "\t\t}\n"
    "\n"
    "\t\tif ((dst_include || dst_exclude) && dst_sent == 0) {\n"
    "\t\t\tchar *xml = dst_build_post_auth_xml(ws, dst_include,\n"
    "\t\t\t\t\t\t\t     dst_exclude);\n"
    "\n"
    "\t\t\tif (xml) {\n"
    "\t\t\t\toclog(ws, LOG_INFO,\n"
    '\t\t\t\t      "adding X-CSTP-Post-Auth-XML include=\'%s\' exclude=\'%s\'",\n'
    '\t\t\t\t      dst_include ? dst_include : "",\n'
    '\t\t\t\t      dst_exclude ? dst_exclude : "");\n'
    '\t\t\t\tret = cstp_printf(ws, "X-CSTP-Post-Auth-XML: %s\\r\\n",\n'
    "\t\t\t\t\t\t  xml);\n"
    "\t\t\t\tSEND_ERR(ret);\n"
    "\t\t\t\ttalloc_free(xml);\n"
    "\t\t\t}\n"
    "\t\t}\n"
    "\t}\n"
)

EMIT_OLD_UPSTREAM = (
    "\tfor (i = 0; i < WSRCONFIG(ws)->n_custom_header; i++) {\n"
    "\t\tchar *h = replace_vals(ws, WSRCONFIG(ws)->custom_header[i]);\n"
    "\n"
    "\t\tif (h) {\n"
    '\t\t\toclog(ws, LOG_INFO, "adding custom header \'%s\'", h);\n'
    '\t\t\tret = cstp_printf(ws, "%s\\r\\n", h);\n'
    "\t\t\tSEND_ERR(ret);\n"
    "\t\t\ttalloc_free(h);\n"
    "\t\t}\n"
    "\t}\n"
)

# Phase-1 emit (include-only) — upgrade target
EMIT_OLD_PHASE1_MARK = "\t\tchar *dst_domains = NULL;\n"


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit(f"usage: {sys.argv[0]} <ocserv-src>")
    root = Path(sys.argv[1])
    worker = root / "src/worker-auth.c"
    if not worker.is_file():
        raise SystemExit(f"{root}: not an ocserv source tree")
    if "parse_group_access_url" not in worker.read_text():
        raise SystemExit(
            f"{root}: SPEC-01 not applied (missing parse_group_access_url); "
            "run scripts/apply-spec01-edits.py first"
        )

    # --- ipc.proto ---
    ipc = root / "src/ipc.proto"
    text = ipc.read_text()
    if "dynamic_split_include_domains" not in text:
        text = replace_once(
            text,
            "\trepeated string split_dns = 41;\n"
            "\toptional uint32 client_bypass_protocol = 42;\n"
            "}\n",
            "\trepeated string split_dns = 41;\n"
            "\toptional uint32 client_bypass_protocol = 42;\n"
            "\trepeated string dynamic_split_include_domains = 43; /* SPEC-02 DST */\n"
            "\trepeated string dynamic_split_exclude_domains = 44; /* SPEC-02 DST */\n"
            "}\n",
            "ipc.proto group_cfg_st",
        )
    elif "dynamic_split_exclude_domains" not in text:
        text = replace_once(
            text,
            "\trepeated string dynamic_split_include_domains = 43; /* SPEC-02 DST */\n"
            "}\n",
            "\trepeated string dynamic_split_include_domains = 43; /* SPEC-02 DST */\n"
            "\trepeated string dynamic_split_exclude_domains = 44; /* SPEC-02 DST */\n"
            "}\n",
            "ipc.proto exclude",
        )
    else:
        print("ipc.proto: already present")
    ipc.write_text(text)

    # --- cfg.proto ---
    cfgp = root / "src/cfg.proto"
    text = cfgp.read_text()
    if "dynamic_split_include_domains" not in text:
        text = replace_once(
            text,
            "  repeated string split_dns             = 203; /* [scope: vhost user] */\n"
            "  repeated string included_http_headers = 204; /* [scope: vhost] */\n",
            "  repeated string split_dns             = 203; /* [scope: vhost user] */\n"
            "  repeated string dynamic_split_include_domains = 206; /* [scope: vhost user] SPEC-02 */\n"
            "  repeated string dynamic_split_exclude_domains = 207; /* [scope: vhost user] SPEC-02 */\n"
            "  repeated string included_http_headers = 204; /* [scope: vhost] */\n",
            "cfg.proto",
        )
    elif "dynamic_split_exclude_domains" not in text:
        text = replace_once(
            text,
            "  repeated string dynamic_split_include_domains = 206; /* [scope: vhost user] SPEC-02 */\n",
            "  repeated string dynamic_split_include_domains = 206; /* [scope: vhost user] SPEC-02 */\n"
            "  repeated string dynamic_split_exclude_domains = 207; /* [scope: vhost user] SPEC-02 */\n",
            "cfg.proto exclude",
        )
    else:
        print("cfg.proto: already present")
    cfgp.write_text(text)

    # --- config.c ---
    cfgc = root / "src/config.c"
    text = cfgc.read_text()
    if "dynamic-split-include-domains" not in text:
        text = replace_once(
            text,
            '\t} else if (strcmp(name, "split-dns") == 0) {\n'
            "\t\tREAD_MULTI_LINE(config->split_dns, config->n_split_dns);\n",
            '\t} else if (strcmp(name, "split-dns") == 0) {\n'
            "\t\tREAD_MULTI_LINE(config->split_dns, config->n_split_dns);\n"
            '\t} else if (strcmp(name, "dynamic-split-include-domains") == 0) {\n'
            "\t\tREAD_MULTI_LINE(config->dynamic_split_include_domains,\n"
            "\t\t\t\tconfig->n_dynamic_split_include_domains);\n"
            '\t} else if (strcmp(name, "dynamic-split-exclude-domains") == 0) {\n'
            "\t\tREAD_MULTI_LINE(config->dynamic_split_exclude_domains,\n"
            "\t\t\t\tconfig->n_dynamic_split_exclude_domains);\n",
            "config.c parse",
        )
    elif "dynamic-split-exclude-domains" not in text:
        text = replace_once(
            text,
            '\t} else if (strcmp(name, "dynamic-split-include-domains") == 0) {\n'
            "\t\tREAD_MULTI_LINE(config->dynamic_split_include_domains,\n"
            "\t\t\t\tconfig->n_dynamic_split_include_domains);\n",
            '\t} else if (strcmp(name, "dynamic-split-include-domains") == 0) {\n'
            "\t\tREAD_MULTI_LINE(config->dynamic_split_include_domains,\n"
            "\t\t\t\tconfig->n_dynamic_split_include_domains);\n"
            '\t} else if (strcmp(name, "dynamic-split-exclude-domains") == 0) {\n'
            "\t\tREAD_MULTI_LINE(config->dynamic_split_exclude_domains,\n"
            "\t\t\t\tconfig->n_dynamic_split_exclude_domains);\n",
            "config.c exclude",
        )
    else:
        print("config.c: already present")
    cfgc.write_text(text)

    # --- sup-config/file.c ---
    sfile = root / "src/sup-config/file.c"
    text = sfile.read_text()
    if "dynamic-split-include-domains" not in text:
        text = replace_once(
            text,
            '\t} else if (strcmp(name, "split-dns") == 0) {\n'
            "\t\tREAD_RAW_MULTI_LINE(msg->config->split_dns,\n"
            "\t\t\t\t    msg->config->n_split_dns);\n",
            '\t} else if (strcmp(name, "split-dns") == 0) {\n'
            "\t\tREAD_RAW_MULTI_LINE(msg->config->split_dns,\n"
            "\t\t\t\t    msg->config->n_split_dns);\n"
            '\t} else if (strcmp(name, "dynamic-split-include-domains") == 0) {\n'
            "\t\tREAD_RAW_MULTI_LINE(msg->config->dynamic_split_include_domains,\n"
            "\t\t\t\t    msg->config->n_dynamic_split_include_domains);\n"
            '\t} else if (strcmp(name, "dynamic-split-exclude-domains") == 0) {\n'
            "\t\tREAD_RAW_MULTI_LINE(msg->config->dynamic_split_exclude_domains,\n"
            "\t\t\t\t    msg->config->n_dynamic_split_exclude_domains);\n",
            "sup-config/file.c parse",
        )
    elif "dynamic-split-exclude-domains" not in text:
        text = replace_once(
            text,
            '\t} else if (strcmp(name, "dynamic-split-include-domains") == 0) {\n'
            "\t\tREAD_RAW_MULTI_LINE(msg->config->dynamic_split_include_domains,\n"
            "\t\t\t\t    msg->config->n_dynamic_split_include_domains);\n",
            '\t} else if (strcmp(name, "dynamic-split-include-domains") == 0) {\n'
            "\t\tREAD_RAW_MULTI_LINE(msg->config->dynamic_split_include_domains,\n"
            "\t\t\t\t    msg->config->n_dynamic_split_include_domains);\n"
            '\t} else if (strcmp(name, "dynamic-split-exclude-domains") == 0) {\n'
            "\t\tREAD_RAW_MULTI_LINE(msg->config->dynamic_split_exclude_domains,\n"
            "\t\t\t\t    msg->config->n_dynamic_split_exclude_domains);\n",
            "sup-config/file.c exclude",
        )
    else:
        print("sup-config/file.c: already present")
    sfile.write_text(text)

    # --- main-sec-mod-cmd.c ---
    msec = root / "src/main-sec-mod-cmd.c"
    text = msec.read_text()
    if "dynamic_split_include_domains" not in text:
        text = replace_once(
            text,
            "\tif (gc->split_dns == NULL) {\n"
            "\t\tgc->split_dns = vhost->config->split_dns;\n"
            "\t\tgc->n_split_dns = vhost->config->n_split_dns;\n"
            "\t}\n",
            "\tif (gc->split_dns == NULL) {\n"
            "\t\tgc->split_dns = vhost->config->split_dns;\n"
            "\t\tgc->n_split_dns = vhost->config->n_split_dns;\n"
            "\t}\n"
            "\n"
            "\tif (gc->dynamic_split_include_domains == NULL) {\n"
            "\t\tgc->dynamic_split_include_domains =\n"
            "\t\t\tvhost->config->dynamic_split_include_domains;\n"
            "\t\tgc->n_dynamic_split_include_domains =\n"
            "\t\t\tvhost->config->n_dynamic_split_include_domains;\n"
            "\t}\n"
            "\n"
            "\tif (gc->dynamic_split_exclude_domains == NULL) {\n"
            "\t\tgc->dynamic_split_exclude_domains =\n"
            "\t\t\tvhost->config->dynamic_split_exclude_domains;\n"
            "\t\tgc->n_dynamic_split_exclude_domains =\n"
            "\t\t\tvhost->config->n_dynamic_split_exclude_domains;\n"
            "\t}\n",
            "main-sec-mod-cmd.c defaults",
        )
    elif "dynamic_split_exclude_domains" not in text:
        text = replace_once(
            text,
            "\tif (gc->dynamic_split_include_domains == NULL) {\n"
            "\t\tgc->dynamic_split_include_domains =\n"
            "\t\t\tvhost->config->dynamic_split_include_domains;\n"
            "\t\tgc->n_dynamic_split_include_domains =\n"
            "\t\t\tvhost->config->n_dynamic_split_include_domains;\n"
            "\t}\n",
            "\tif (gc->dynamic_split_include_domains == NULL) {\n"
            "\t\tgc->dynamic_split_include_domains =\n"
            "\t\t\tvhost->config->dynamic_split_include_domains;\n"
            "\t\tgc->n_dynamic_split_include_domains =\n"
            "\t\t\tvhost->config->n_dynamic_split_include_domains;\n"
            "\t}\n"
            "\n"
            "\tif (gc->dynamic_split_exclude_domains == NULL) {\n"
            "\t\tgc->dynamic_split_exclude_domains =\n"
            "\t\t\tvhost->config->dynamic_split_exclude_domains;\n"
            "\t\tgc->n_dynamic_split_exclude_domains =\n"
            "\t\t\tvhost->config->n_dynamic_split_exclude_domains;\n"
            "\t}\n",
            "main-sec-mod-cmd.c exclude",
        )
    else:
        print("main-sec-mod-cmd.c: already present")
    msec.write_text(text)

    # --- worker-vpn.c helpers + emit ---
    wvpn = root / "src/worker-vpn.c"
    text = wvpn.read_text()
    helper_mark = "/* SPEC-02: Dynamic Split Tunneling helpers"
    connect_anchor = (
        "/* connect_handler:\n"
        " * @ws: an initialized worker structure\n"
    )

    if "dst_custom_attr_xml" in text and "dynamic_split_exclude_domains" in text:
        print("worker-vpn.c helpers: already present (phase2)")
    elif helper_mark in text:
        # Upgrade Phase-1 helpers → Phase-2
        start = text.find(helper_mark)
        end = text.find(connect_anchor, start)
        if start < 0 or end < 0:
            raise SystemExit("worker-vpn.c helpers: cannot locate block to upgrade")
        text = text[:start] + WORKER_DST_HELPERS + text[end:]
        print("worker-vpn.c helpers: upgraded to phase2")
    else:
        text = insert_before(
            text, connect_anchor, WORKER_DST_HELPERS, "worker-vpn.c helpers"
        )

    if "dst_exclude" in text and "n_dynamic_split_exclude_domains" in text:
        print("worker-vpn.c emit: already present (phase2)")
    elif EMIT_OLD_PHASE1_MARK in text:
        # Replace Phase-1 emit block: from "\t{\n\t\tchar *dst_domains" through matching close
        start = text.find("\t{\n" + EMIT_OLD_PHASE1_MARK)
        if start < 0:
            raise SystemExit("worker-vpn.c emit: phase1 block not found")
        # Find end: after phase1 block comes "\t/* set TCP socket options */"
        end_anchor = "\t/* set TCP socket options */\n"
        end = text.find(end_anchor, start)
        if end < 0:
            raise SystemExit("worker-vpn.c emit: end anchor not found")
        text = text[:start] + EMIT_BLOCK + "\n" + text[end:]
        print("worker-vpn.c emit: upgraded to phase2")
    elif "dst_sent" not in text:
        text = replace_once(text, EMIT_OLD_UPSTREAM, EMIT_BLOCK, "worker-vpn.c emit")
    else:
        raise SystemExit("worker-vpn.c emit: unexpected DST state")
    wvpn.write_text(text)

    # --- sample.config ---
    sample = root / "doc/sample.config"
    if sample.is_file():
        text = sample.read_text()
        if "dynamic-split-include-domains" not in text:
            text = insert_after(
                text,
                "#split-dns = example.com\n",
                "\n"
                "# Cisco AnyConnect Dynamic Split Tunneling (SPEC-02).\n"
                "# Include: resolved IPs added to tunnel routes (split-include).\n"
                "# Exclude: resolved IPs kept local under tunnel-all (route = default).\n"
                "# Both set = Enhanced DST (AnyConnect >= 4.6).\n"
                "# Comma-separated and/or repeated lines; merged global→group→user.\n"
                "# [scope: vhost user]\n"
                "#dynamic-split-include-domains = corp.example.com, app.example.com\n"
                "#dynamic-split-exclude-domains = www.cisco.com, tools.cisco.com\n",
                "doc/sample.config",
            )
        elif "dynamic-split-exclude-domains" not in text:
            text = insert_after(
                text,
                "#dynamic-split-include-domains = corp.example.com, app.example.com\n",
                "#dynamic-split-exclude-domains = www.cisco.com, tools.cisco.com\n",
                "doc/sample.config exclude",
            )
        else:
            print("doc/sample.config: already present")
        sample.write_text(text)

    print("SPEC-02 DST edits applied OK (include+exclude / Enhanced)")


if __name__ == "__main__":
    main()
