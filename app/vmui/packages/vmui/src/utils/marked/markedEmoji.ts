import { MarkedExtension, RendererThis, Tokens } from "marked";

interface EmojiToken<T> extends Tokens.Generic {
  type: "emoji";
  raw: string;
  name: string;
  emoji: T;
}

type MarkedEmojiOptions<T> = {
  emojis: Record<string, T>;
  renderer(token: EmojiToken<T>): string;
};

function markedEmoji<T>(options: MarkedEmojiOptions<T>): MarkedExtension {
  const { emojis } = options;
  if (!emojis) {
    throw new Error("Must provide emojis to markedEmoji");
  }

  const emojiNames = Object.keys(emojis)
    .map(e => e.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
    .join("|");

  if (emojiNames.length === 0) {
    throw new Error("Emoji list is empty; provide at least one emoji.");
  }

  const emojiRegex = new RegExp(`:(${emojiNames}):`);
  const tokenizerRule = new RegExp(`^${emojiRegex.source}`);

  return {
    extensions: [{
      name: "emoji",
      level: "inline",
      start(src: string) {
        return src.match(emojiRegex)?.index;
      },
      tokenizer(src: string) {
        const match = tokenizerRule.exec(src);
        if (!match) {
          return;
        }

        const name = match[1];
        const emoji = emojis[name];

        if (!emoji) {
          return;
        }

        return {
          type: "emoji",
          raw: match[0],
          name,
          emoji,
        };
      },
      renderer(this: RendererThis, token: Tokens.Generic): string {
        return options.renderer(token as EmojiToken<T>);
      }
    }],
  };
}

export default markedEmoji;
