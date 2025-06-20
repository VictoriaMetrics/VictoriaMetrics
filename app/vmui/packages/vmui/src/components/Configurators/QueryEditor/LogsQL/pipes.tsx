import { ContextType } from "./types";
import { FunctionIcon } from "../../../Main/Icons";

const docsUrl = "https://docs.victoriametrics.com/victorialogs/logsql";
const classLink = "vm-link vm-link_colored";

const prepareDescription = (text: string): string => {
  const replaceClass = `$1 target="_blank" class="${classLink}" $2`;
  const replaceHref = `$1 $2${docsUrl}#`;
  return text
    .replace(/(<a) (href=")#/gm, replaceHref)
    .replace(/(<a) (href="[^"]+")/gm, replaceClass);
};

export const pipeList = [
  {
    "value": "copy",
    "description": "<a href=\"#copy-pipe\"><code>copy</code></a> copies <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a>."
  },
  {
    "value": "delete",
    "description": "<a href=\"#delete-pipe\"><code>delete</code></a> deletes <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a>."
  },
  {
    "value": "drop_empty_fields",
    "description": "<a href=\"#drop_empty_fields-pipe\"><code>drop_empty_fields</code></a> drops <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a> with empty values."
  },
  {
    "value": "extract",
    "description": "<a href=\"#extract-pipe\"><code>extract</code></a> extracts the specified text into the given log fields."
  },
  {
    "value": "extract_regexp",
    "description": "<a href=\"#extract_regexp-pipe\"><code>extract_regexp</code></a> extracts the specified text into the given log fields via <a href=\"https://github.com/google/re2/wiki/Syntax\" rel=\"external\" target=\"_blank\">RE2 regular expressions</a>."
  },
  {
    "value": "field_names",
    "description": "<a href=\"#field_names-pipe\"><code>field_names</code></a> returns all the names of <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a>."
  },
  {
    "value": "field_values",
    "description": "<a href=\"#field_values-pipe\"><code>field_values</code></a> returns all the values for the given <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log field</a>."
  },
  {
    "value": "fields",
    "description": "<a href=\"#fields-pipe\"><code>fields</code></a> selects the given set of <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a>."
  },
  {
    "value": "filter",
    "description": "<a href=\"#filter-pipe\"><code>filter</code></a> applies additional <a href=\"#filters\">filters</a> to results."
  },
  {
    "value": "format",
    "description": "<a href=\"#format-pipe\"><code>format</code></a> formats output field from input <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a>."
  },
  {
    "value": "limit",
    "description": "<a href=\"#limit-pipe\"><code>limit</code></a> limits the number selected logs."
  },
  {
    "value": "math",
    "description": "<a href=\"#math-pipe\"><code>math</code></a> performs mathematical calculations over <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a>."
  },
  {
    "value": "offset",
    "description": "<a href=\"#offset-pipe\"><code>offset</code></a> skips the given number of selected logs."
  },
  {
    "value": "pack_json",
    "description": "<a href=\"#pack_json-pipe\"><code>pack_json</code></a> packs <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a> into JSON object."
  },
  {
    "value": "pack_logfmt",
    "description": "<a href=\"#pack_logfmt-pipe\"><code>pack_logfmt</code></a> packs <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a> into <a href=\"https://brandur.org/logfmt\" rel=\"external\" target=\"_blank\">logfmt</a> message."
  },
  {
    "value": "rename",
    "description": "<a href=\"#rename-pipe\"><code>rename</code></a> renames <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a>."
  },
  {
    "value": "replace",
    "description": "<a href=\"#replace-pipe\"><code>replace</code></a> replaces substrings in the specified <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a>."
  },
  {
    "value": "replace_regexp",
    "description": "<a href=\"#replace_regexp-pipe\"><code>replace_regexp</code></a> updates <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a> with regular expressions."
  },
  {
    "value": "sort",
    "description": "<a href=\"#sort-pipe\"><code>sort</code></a> sorts logs by the given <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">fields</a>."
  },
  {
    "value": "stats",
    "description": "<a href=\"#stats-pipe\"><code>stats</code></a> calculates various stats over the selected logs."
  },
  {
    "value": "stream_context",
    "description": "<a href=\"#stream_context-pipe\"><code>stream_context</code></a> allows selecting surrounding logs in front and after the matching logs\nper each <a href=\"/victorialogs/keyconcepts/#stream-fields\">log stream</a>."
  },
  {
    "value": "top",
    "description": "<a href=\"#top-pipe\"><code>top</code></a> returns top <code>N</code> field sets with the maximum number of matching logs."
  },
  {
    "value": "uniq",
    "description": "<a href=\"#uniq-pipe\"><code>uniq</code></a> returns unique log entries."
  },
  {
    "value": "unpack_json",
    "description": "<a href=\"#unpack_json-pipe\"><code>unpack_json</code></a> unpacks JSON messages from <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a>."
  },
  {
    "value": "unpack_logfmt",
    "description": "<a href=\"#unpack_logfmt-pipe\"><code>unpack_logfmt</code></a> unpacks <a href=\"https://brandur.org/logfmt\" rel=\"external\" target=\"_blank\">logfmt</a> messages from <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a>."
  },
  {
    "value": "unpack_syslog",
    "description": "<a href=\"#unpack_syslog-pipe\"><code>unpack_syslog</code></a> unpacks <a href=\"https://en.wikipedia.org/wiki/Syslog\" rel=\"external\" target=\"_blank\">syslog</a> messages from <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a>."
  },
  {
    "value": "unroll",
    "description": "<a href=\"#unroll-pipe\"><code>unroll</code></a> unrolls JSON arrays from <a href=\"https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model\">log fields</a>."
  }
].map(item => ({
  ...item,
  type: ContextType.PipeName,
  icon: <FunctionIcon/>,
  description: prepareDescription(item.description),
}));
