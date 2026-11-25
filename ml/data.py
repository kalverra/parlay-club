"""Historical NFL data loading, caching, team/position normalization, and joining via nflreadpy."""

from __future__ import annotations

from pathlib import Path
import re
from typing import Iterable

import nflreadpy as nfl
import numpy as np
import polars as pl

DEFAULT_CACHE_DIR = Path(__file__).resolve().parent / "cache"
GAMES_CSV_URL = "https://github.com/nflverse/nfldata/raw/master/data/games.csv"

# Mirrors Go internal/ingest/nflverse/teams.go
TEAM_CODES: dict[str, str] = {
    "JAC": "JAX",
    "LA": "LAR",
    "OAK": "LV",
    "SD": "LAC",
    "STL": "LAR",
    "WAS": "WSH",
}

SEASON_TYPE_REG = 2
SEASON_TYPE_POST = 3


def map_team(code: str | None) -> str:
    if not code:
        return ""
    cleaned = code.strip().upper()
    return TEAM_CODES.get(cleaned, cleaned)


def normalize_position(pos: str | None) -> str:
    if not pos:
        return ""
    cleaned = pos.strip().upper()
    if cleaned in ("FB", "HB"):
        return "RB"
    return cleaned


def _normalize_name(name: str | None) -> str:
    if not name:
        return ""
    return re.sub(r"[^a-z0-9]", "", name.lower())


def _map_season_type_expr(col: str = "season_type") -> pl.Expr:
    return (
        pl.when(pl.col(col).cast(pl.Utf8).str.to_uppercase().is_in(["REG", "REGULAR", "2"]))
        .then(SEASON_TYPE_REG)
        .when(pl.col(col).cast(pl.Utf8).str.to_uppercase().is_in(["POST", "POSTSEASON", "WC", "DIV", "CON", "SB", "3"]))
        .then(SEASON_TYPE_POST)
        .otherwise(0)
        .alias("season_type")
    )


def _map_team_expr(col: str, alias: str | None = None) -> pl.Expr:
    target = alias or col
    expr = pl.col(col).cast(pl.Utf8).str.strip_chars().str.to_uppercase()
    for src, dst in TEAM_CODES.items():
        expr = pl.when(expr == src).then(pl.lit(dst)).otherwise(expr)
    return expr.fill_null("").alias(target)


def load_schedules(seasons: list[int], cache_dir: Path = DEFAULT_CACHE_DIR, refresh: bool = False) -> pl.DataFrame:
    """Load historical NFL schedules with closing spreads, totals, weather, and scores."""
    cache_dir.mkdir(parents=True, exist_ok=True)
    cache_path = cache_dir / "games.parquet"
    if cache_path.exists() and not refresh:
        games = pl.read_parquet(cache_path)
    else:
        try:
            games = pl.read_csv(GAMES_CSV_URL, infer_schema_length=10000)
        except Exception:
            games = nfl.load_schedules(seasons)
        games.write_parquet(cache_path)

    games = games.filter(pl.col("season").is_in(seasons))
    # Map game_type -> season_type (REG=2, WC/DIV/CON/SB=3)
    games = games.with_columns(
        pl.when(pl.col("game_type") == "REG")
        .then(SEASON_TYPE_REG)
        .when(pl.col("game_type").is_in(["WC", "DIV", "CON", "SB", "POST"]))
        .then(SEASON_TYPE_POST)
        .otherwise(0)
        .alias("season_type"),
        _map_team_expr("home_team"),
        _map_team_expr("away_team"),
    ).filter(pl.col("season_type").is_in([SEASON_TYPE_REG, SEASON_TYPE_POST]))

    # nflverse games.csv spread_line is positive when home is favored; Go MarketScript uses negative = home favored.
    return games.select(
        [
            pl.col("game_id"),
            pl.col("season").cast(pl.Int64),
            pl.col("season_type").cast(pl.Int64),
            pl.col("week").cast(pl.Int64),
            pl.col("home_team"),
            pl.col("away_team"),
            (-pl.col("spread_line").cast(pl.Float64)).alias("home_spread"),
            pl.col("total_line").cast(pl.Float64).alias("game_total"),
            pl.col("roof").cast(pl.Utf8).fill_null("").str.to_lowercase().alias("roof"),
            pl.col("temp").cast(pl.Float64).alias("temp_f"),
            pl.col("wind").cast(pl.Float64).alias("wind_mph"),
            pl.col("home_score").cast(pl.Float64).alias("home_score"),
            pl.col("away_score").cast(pl.Float64).alias("away_score"),
            pl.col("result").cast(pl.Float64).alias("actual_home_margin"),
            pl.col("total").cast(pl.Float64).alias("actual_game_total"),
        ]
    )


def _cached_or_load(
    name: str,
    seasons: list[int],
    loader_fn,
    cache_dir: Path,
    refresh: bool,
) -> pl.DataFrame:
    cache_dir.mkdir(parents=True, exist_ok=True)
    tag = f"{min(seasons)}_{max(seasons)}_{len(seasons)}"
    cache_path = cache_dir / f"{name}_{tag}.parquet"
    if cache_path.exists() and not refresh:
        return pl.read_parquet(cache_path)
    df = loader_fn(seasons)
    df.write_parquet(cache_path)
    return df


def _load_ff_opportunity_safe(seasons: list[int], cache_dir: Path, refresh: bool) -> pl.DataFrame | None:
    frames: list[pl.DataFrame] = []
    for season in seasons:
        cache_path = cache_dir / f"ff_opp_{season}.parquet"
        if cache_path.exists() and not refresh:
            frames.append(pl.read_parquet(cache_path))
            continue
        try:
            df = nfl.load_ff_opportunity(season, stat_type="weekly")
            cols = [
                "season",
                "week",
                "posteam",
                "player_id",
                "receptions_exp",
                "pass_yards_gained_exp",
                "rec_yards_gained_exp",
                "rush_yards_gained_exp",
                "pass_touchdown_exp",
                "rec_touchdown_exp",
                "rush_touchdown_exp",
            ]
            available = [c for c in cols if c in df.columns]
            sub = df.select(available)
            sub.write_parquet(cache_path)
            frames.append(sub)
        except Exception:
            # Fail soft identically to Go Client.FFOpportunity when a season lacks ff_opportunity.
            continue
    if not frames:
        return None
    return pl.concat(frames, how="diagonal_relaxed")


def load_historical_tables(
    seasons: Iterable[int],
    cache_dir: Path = DEFAULT_CACHE_DIR,
    refresh: bool = False,
) -> tuple[pl.DataFrame, pl.DataFrame]:
    """Load, normalize, and join historical player-week and team-week DataFrames from nflreadpy."""
    season_list = sorted({int(s) for s in seasons})
    if not season_list:
        raise ValueError("seasons must not be empty")

    schedules = load_schedules(season_list, cache_dir=cache_dir, refresh=refresh)
    player_stats = _cached_or_load(
        "player_stats",
        season_list,
        lambda s: nfl.load_player_stats(s, summary_level="week"),
        cache_dir=cache_dir,
        refresh=refresh,
    )
    team_stats = _cached_or_load(
        "team_stats",
        season_list,
        lambda s: nfl.load_team_stats(s, summary_level="week"),
        cache_dir=cache_dir,
        refresh=refresh,
    )
    snap_counts = _cached_or_load(
        "snap_counts",
        season_list,
        lambda s: nfl.load_snap_counts(s),
        cache_dir=cache_dir,
        refresh=refresh,
    )
    players_cache = cache_dir / "players.parquet"
    if players_cache.exists() and not refresh:
        players = pl.read_parquet(players_cache)
    else:
        players = nfl.load_players()
        players.write_parquet(players_cache)

    ff_opp = _load_ff_opportunity_safe(season_list, cache_dir=cache_dir, refresh=refresh)

    return assemble_joined_tables(
        player_stats=player_stats,
        team_stats=team_stats,
        snap_counts=snap_counts,
        players=players,
        schedules=schedules,
        ff_opportunity=ff_opp,
    )


def assemble_joined_tables(
    player_stats: pl.DataFrame,
    team_stats: pl.DataFrame,
    snap_counts: pl.DataFrame,
    players: pl.DataFrame,
    schedules: pl.DataFrame,
    ff_opportunity: pl.DataFrame | None = None,
) -> tuple[pl.DataFrame, pl.DataFrame]:
    """Normalize and join player-week and team-week tables cleanly (testable offline without network)."""
    team_df = _normalize_team_stats(team_stats)
    ps_df = _normalize_player_stats(player_stats)
    sc_df = _normalize_snap_counts(snap_counts, players)

    # Join carry_share denominator from team_stats
    team_carries = team_df.select(["season", "season_type", "week", "team", "team_carries"])
    ps_df = ps_df.join(team_carries, on=["season", "season_type", "week", "team"], how="left")
    ps_df = ps_df.with_columns(
        pl.when((pl.col("team_carries").fill_null(0) > 0) & (pl.col("carries") > 0))
        .then(pl.col("carries").cast(pl.Float64) / pl.col("team_carries").cast(pl.Float64))
        .otherwise(0.0)
        .alias("carry_share")
    ).drop("team_carries")

    # Join snap counts via gsis_id (player_id) first, then fallback to (season, season_type, week, team, norm_name)
    if sc_df.height > 0:
        sc_by_id = (
            sc_df.filter(pl.col("gsis_id") != "")
            .group_by(["gsis_id", "season", "season_type", "week"])
            .agg(pl.col("offense_pct").max().alias("offense_pct_id"))
        )
        ps_df = ps_df.join(
            sc_by_id,
            left_on=["player_id", "season", "season_type", "week"],
            right_on=["gsis_id", "season", "season_type", "week"],
            how="left",
        )
        sc_by_name = (
            sc_df.filter(pl.col("norm_name") != "")
            .group_by(["season", "season_type", "week", "team", "norm_name"])
            .agg(pl.col("offense_pct").max().alias("offense_pct_name"))
        )
        ps_df = ps_df.join(
            sc_by_name,
            on=["season", "season_type", "week", "team", "norm_name"],
            how="left",
        )
        ps_df = ps_df.with_columns(
            pl.coalesce([pl.col("offense_pct_id"), pl.col("offense_pct_name"), pl.lit(0.0)]).alias("offense_pct")
        ).drop(["offense_pct_id", "offense_pct_name"])
    else:
        ps_df = ps_df.with_columns(pl.lit(0.0).alias("offense_pct"))

    # Join ff_opportunity expected metrics (failing soft if missing)
    exp_cols = [
        ("receptions_exp", "exp_receptions"),
        ("pass_yards_gained_exp", "exp_pass_yds"),
        ("rec_yards_gained_exp", "exp_rec_yds"),
        ("rush_yards_gained_exp", "exp_rush_yds"),
        ("pass_touchdown_exp", "exp_pass_tds"),
        ("rec_touchdown_exp", "exp_rec_tds"),
        ("rush_touchdown_exp", "exp_rush_tds"),
    ]
    if ff_opportunity is not None and ff_opportunity.height > 0:
        ff = ff_opportunity.with_columns(
            pl.col("season").cast(pl.Int64),
            pl.col("week").cast(pl.Int64),
            pl.col("player_id").cast(pl.Utf8),
        )
        select_exprs = [pl.col("season"), pl.col("week"), pl.col("player_id")]
        for src, dst in exp_cols:
            if src in ff.columns:
                select_exprs.append(pl.col(src).cast(pl.Float64).fill_null(0.0).alias(dst))
            else:
                select_exprs.append(pl.lit(0.0).alias(dst))
        ff_agg = ff.select(select_exprs).group_by(["season", "week", "player_id"]).agg(
            [pl.col(dst).mean().alias(dst) for _, dst in exp_cols]
        )
        ps_df = ps_df.join(ff_agg, on=["season", "week", "player_id"], how="left")
    for _, dst in exp_cols:
        if dst not in ps_df.columns:
            ps_df = ps_df.with_columns(pl.lit(0.0).alias(dst))
        else:
            ps_df = ps_df.with_columns(pl.col(dst).fill_null(0.0))

    # Join game-level context from schedules onto player rows
    ps_df = _attach_schedule_context(ps_df, schedules)
    ps_df = ps_df.drop("norm_name")
    return ps_df, team_df


def _normalize_team_stats(team_stats: pl.DataFrame) -> pl.DataFrame:
    df = team_stats.with_columns(
        pl.col("season").cast(pl.Int64),
        pl.col("week").cast(pl.Int64),
        _map_season_type_expr("season_type"),
        _map_team_expr("team"),
        _map_team_expr("opponent_team", "opponent"),
        pl.col("passing_epa").cast(pl.Float64).fill_null(0.0).alias("off_pass_epa"),
        pl.col("rushing_epa").cast(pl.Float64).fill_null(0.0).alias("off_rush_epa"),
        pl.col("attempts").cast(pl.Int64).fill_null(0).alias("team_attempts"),
        pl.col("carries").cast(pl.Int64).fill_null(0).alias("team_carries"),
    ).filter(
        pl.col("season_type").is_in([SEASON_TYPE_REG, SEASON_TYPE_POST])
        & (pl.col("team") != "")
        & (pl.col("opponent") != "")
    )
    df = df.with_columns(
        (pl.col("team_attempts") + pl.col("team_carries")).cast(pl.Float64).alias("plays_per_game"),
        pl.when((pl.col("team_attempts") + pl.col("team_carries")) > 0)
        .then(pl.col("team_attempts").cast(pl.Float64) / (pl.col("team_attempts") + pl.col("team_carries")).cast(pl.Float64))
        .otherwise(0.0)
        .alias("pass_rate"),
    )

    opp_df = df.select(
        [
            pl.col("season"),
            pl.col("season_type"),
            pl.col("week"),
            pl.col("team").alias("opponent"),
            pl.col("off_pass_epa").alias("def_pass_epa_allowed"),
            pl.col("off_rush_epa").alias("def_rush_epa_allowed"),
        ]
    )
    return df.join(opp_df, on=["season", "season_type", "week", "opponent"], how="left").with_columns(
        pl.col("def_pass_epa_allowed").fill_null(0.0),
        pl.col("def_rush_epa_allowed").fill_null(0.0),
    )


def _normalize_player_stats(player_stats: pl.DataFrame) -> pl.DataFrame:
    name_col = "player_display_name" if "player_display_name" in player_stats.columns else "player_name"
    cpoe_expr = (
        pl.col("passing_cpoe").cast(pl.Float64).fill_null(0.0).alias("cpoe")
        if "passing_cpoe" in player_stats.columns
        else pl.lit(0.0).alias("cpoe")
    )
    df = player_stats.with_columns(
        pl.col("player_id").cast(pl.Utf8).fill_null(""),
        pl.col(name_col).cast(pl.Utf8).fill_null("").alias("player_name"),
        pl.col("position").cast(pl.Utf8).fill_null("").str.strip_chars().str.to_uppercase().alias("raw_pos"),
        _map_team_expr("team"),
        _map_team_expr("opponent_team", "opponent"),
        pl.col("season").cast(pl.Int64),
        pl.col("week").cast(pl.Int64),
        _map_season_type_expr("season_type"),
        pl.col("target_share").cast(pl.Float64).fill_null(0.0),
        pl.col("air_yards_share").cast(pl.Float64).fill_null(0.0),
        pl.col("wopr").cast(pl.Float64).fill_null(0.0),
        (
            pl.col("passing_epa").cast(pl.Float64).fill_null(0.0)
            + pl.col("rushing_epa").cast(pl.Float64).fill_null(0.0)
            + pl.col("receiving_epa").cast(pl.Float64).fill_null(0.0)
        ).alias("total_epa"),
        cpoe_expr,
        pl.col("attempts").cast(pl.Int64).fill_null(0).alias("pass_att"),
        pl.col("carries").cast(pl.Int64).fill_null(0),
        pl.col("targets").cast(pl.Int64).fill_null(0),
        pl.col("receptions").cast(pl.Int64).fill_null(0),
        pl.col("passing_yards").cast(pl.Float64).fill_null(0.0).alias("pass_yds"),
        pl.col("rushing_yards").cast(pl.Float64).fill_null(0.0).alias("rush_yds"),
        pl.col("receiving_yards").cast(pl.Float64).fill_null(0.0).alias("rec_yds"),
        pl.col("passing_tds").cast(pl.Int64).fill_null(0).alias("pass_tds"),
        pl.col("rushing_tds").cast(pl.Int64).fill_null(0).alias("rush_tds"),
        pl.col("receiving_tds").cast(pl.Int64).fill_null(0).alias("rec_tds"),
    ).with_columns(
        pl.when(pl.col("raw_pos").is_in(["FB", "HB"]))
        .then(pl.lit("RB"))
        .otherwise(pl.col("raw_pos"))
        .alias("position"),
        pl.col("player_name").map_elements(_normalize_name, return_dtype=pl.Utf8).alias("norm_name"),
        (
            (pl.col("rushing_tds").cast(pl.Int64).fill_null(0) + pl.col("receiving_tds").cast(pl.Int64).fill_null(0)) > 0
        )
        .cast(pl.Float64)
        .alias("anytime_td"),
        pl.lit(0.0).alias("rz_opps"),
    )

    return df.filter(
        pl.col("season_type").is_in([SEASON_TYPE_REG, SEASON_TYPE_POST])
        & (pl.col("player_id") != "")
        & (pl.col("team") != "")
    ).select(
        [
            "player_id",
            "player_name",
            "norm_name",
            "position",
            "team",
            "opponent",
            "season",
            "season_type",
            "week",
            "target_share",
            "air_yards_share",
            "wopr",
            "total_epa",
            "cpoe",
            "pass_att",
            "carries",
            "targets",
            "receptions",
            "pass_yds",
            "rush_yds",
            "rec_yds",
            "pass_tds",
            "rush_tds",
            "rec_tds",
            "anytime_td",
            "rz_opps",
        ]
    )


def _normalize_snap_counts(snap_counts: pl.DataFrame, players: pl.DataFrame) -> pl.DataFrame:
    if snap_counts.height == 0:
        return pl.DataFrame(
            schema={
                "gsis_id": pl.Utf8,
                "season": pl.Int64,
                "season_type": pl.Int64,
                "week": pl.Int64,
                "team": pl.Utf8,
                "norm_name": pl.Utf8,
                "offense_pct": pl.Float64,
            }
        )
    pfr_map = (
        players.select([pl.col("pfr_id").cast(pl.Utf8), pl.col("gsis_id").cast(pl.Utf8)])
        .filter((pl.col("pfr_id").fill_null("") != "") & (pl.col("gsis_id").fill_null("") != ""))
        .unique(subset=["pfr_id"])
    )
    df = snap_counts.with_columns(
        pl.col("pfr_player_id").cast(pl.Utf8).fill_null("").alias("pfr_id"),
        pl.col("player").cast(pl.Utf8).fill_null("").map_elements(_normalize_name, return_dtype=pl.Utf8).alias("norm_name"),
        _map_team_expr("team"),
        pl.col("season").cast(pl.Int64),
        pl.col("week").cast(pl.Int64),
        _map_season_type_expr("game_type"),
        pl.col("offense_pct").cast(pl.Float64).fill_null(0.0),
    ).filter(pl.col("season_type").is_in([SEASON_TYPE_REG, SEASON_TYPE_POST]) & (pl.col("team") != ""))

    return df.join(pfr_map, on="pfr_id", how="left").with_columns(pl.col("gsis_id").fill_null("")).select(
        ["gsis_id", "season", "season_type", "week", "team", "norm_name", "offense_pct"]
    )


def _attach_schedule_context(ps_df: pl.DataFrame, schedules: pl.DataFrame) -> pl.DataFrame:
    home_rows = schedules.select(
        [
            pl.col("game_id"),
            pl.col("season"),
            pl.col("season_type"),
            pl.col("week"),
            pl.col("home_team").alias("team"),
            pl.col("away_team").alias("sched_opponent"),
            pl.lit(True).alias("is_home"),
            pl.col("home_spread").alias("team_spread"),
            pl.col("home_spread"),
            pl.col("game_total"),
            pl.col("roof"),
            pl.col("temp_f"),
            pl.col("wind_mph"),
            pl.col("actual_home_margin").alias("actual_team_margin"),
            pl.col("actual_game_total"),
        ]
    )
    away_rows = schedules.select(
        [
            pl.col("game_id"),
            pl.col("season"),
            pl.col("season_type"),
            pl.col("week"),
            pl.col("away_team").alias("team"),
            pl.col("home_team").alias("sched_opponent"),
            pl.lit(False).alias("is_home"),
            (-pl.col("home_spread")).alias("team_spread"),
            pl.col("home_spread"),
            pl.col("game_total"),
            pl.col("roof"),
            pl.col("temp_f"),
            pl.col("wind_mph"),
            (-pl.col("actual_home_margin")).alias("actual_team_margin"),
            pl.col("actual_game_total"),
        ]
    )
    sched_team = pl.concat([home_rows, away_rows], how="vertical").unique(
        subset=["season", "season_type", "week", "team"]
    )

    joined = ps_df.join(sched_team, on=["season", "season_type", "week", "team"], how="left")
    return joined.with_columns(
        pl.when(pl.col("opponent") == "")
        .then(pl.col("sched_opponent").fill_null(""))
        .otherwise(pl.col("opponent"))
        .alias("opponent"),
        pl.col("team_spread").abs().alias("abs_spread"),
        ((pl.col("game_total") - pl.col("team_spread")) / 2.0).alias("team_implied_total"),
        ((pl.col("game_total") + pl.col("team_spread")) / 2.0).alias("opp_implied_total"),
        pl.when(pl.col("roof").is_in(["dome", "closed"]))
        .then(1.0)
        .when(pl.col("roof") != "")
        .then(0.0)
        .otherwise(np.nan)
        .alias("is_indoor"),
        pl.when(pl.col("roof").is_in(["dome", "closed"]))
        .then(0.0)
        .otherwise(pl.col("wind_mph"))
        .alias("wind_mph"),
        pl.when(pl.col("roof").is_in(["dome", "closed"]))
        .then(0.0)
        .otherwise(np.nan)
        .alias("precip_in"),
        pl.when(pl.col("roof").is_in(["dome", "closed"]))
        .then(70.0)
        .otherwise(pl.col("temp_f"))
        .alias("temp_f"),
    ).drop("sched_opponent")
