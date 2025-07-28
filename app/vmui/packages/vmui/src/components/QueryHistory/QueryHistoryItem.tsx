import { FC, useMemo } from "preact/compat";
import Button from "../Main/Button/Button";
import { CopyIcon, PlayCircleOutlineIcon, StarBorderIcon, StarIcon } from "../Main/Icons";
import Tooltip from "../Main/Tooltip/Tooltip";
import useCopyToClipboard from "../../hooks/useCopyToClipboard";
import "./style.scss";

interface Props {
  query: string;
  favorites: string[];
  onRun: (query: string) => void;
  onToggleFavorite: (query: string, isFavorite: boolean) => void;
}

const QueryHistoryItem: FC<Props> = ({ query, favorites, onRun, onToggleFavorite }) => {
  const copyToClipboard = useCopyToClipboard();
  const isFavorite = useMemo(() => favorites.includes(query), [query, favorites]);

  const handleCopyQuery = async () => {
    await copyToClipboard(query, "Query has been copied");
  };

  const handleRunQuery = () => {
    onRun(query);
  };

  const handleToggleFavorite = () => {
    onToggleFavorite(query, isFavorite);
  };

  return (
    <div className="vm-query-history-item">
      <span className="vm-query-history-item__value">{query}</span>
      <div className="vm-query-history-item__buttons">
        <Tooltip title={"Execute query"}>
          <Button
            size="small"
            variant="text"
            onClick={handleRunQuery}
            startIcon={<PlayCircleOutlineIcon/>}
          />
        </Tooltip>
        <Tooltip title={"Copy query"}>
          <Button
            size="small"
            variant="text"
            onClick={handleCopyQuery}
            startIcon={<CopyIcon/>}
          />
        </Tooltip>
        <Tooltip title={isFavorite ? "Remove Favorite" : "Add to Favorites"}>
          <Button
            size="small"
            variant="text"
            color={isFavorite ? "warning" : "primary"}
            onClick={handleToggleFavorite}
            startIcon={isFavorite ? <StarIcon/> : <StarBorderIcon/>}
          />
        </Tooltip>
      </div>
    </div>
  );
};

export default QueryHistoryItem;
