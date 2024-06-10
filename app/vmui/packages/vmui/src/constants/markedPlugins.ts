import { markedEmoji } from "marked-emoji";
import { marked } from "marked";
import emojis from "./emojis";

marked.use(markedEmoji({ emojis, renderer: (token) => token.emoji }));
