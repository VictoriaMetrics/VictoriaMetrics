import markedEmoji from "../utils/marked/markedEmoji";
import { marked } from "marked";
import emojis from "./emojis";

// TODO: Dynamically import the emoji map only if the emoji parser is active
marked.use(markedEmoji({ emojis, renderer: (token) => token.emoji }));
