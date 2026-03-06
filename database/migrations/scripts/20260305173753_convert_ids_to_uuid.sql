-- +goose Up
-- +goose StatementBegin

-- =============================================================================
-- PHASE 1: Add new UUID columns to the 5 parent tables
-- =============================================================================
ALTER TABLE teams ADD COLUMN new_id uuid DEFAULT gen_random_uuid() NOT NULL;
ALTER TABLE teams ADD CONSTRAINT teams_new_id_unique UNIQUE (new_id);

ALTER TABLE individuals ADD COLUMN new_id uuid DEFAULT gen_random_uuid() NOT NULL;
ALTER TABLE individuals ADD CONSTRAINT individuals_new_id_unique UNIQUE (new_id);

ALTER TABLE games ADD COLUMN new_id uuid DEFAULT gen_random_uuid() NOT NULL;
ALTER TABLE games ADD CONSTRAINT games_new_id_unique UNIQUE (new_id);

ALTER TABLE nfl_markets ADD COLUMN new_id uuid DEFAULT gen_random_uuid() NOT NULL;
ALTER TABLE nfl_markets ADD CONSTRAINT nfl_markets_new_id_unique UNIQUE (new_id);

ALTER TABLE nba_markets ADD COLUMN new_id uuid DEFAULT gen_random_uuid() NOT NULL;
ALTER TABLE nba_markets ADD CONSTRAINT nba_markets_new_id_unique UNIQUE (new_id);

-- =============================================================================
-- PHASE 2: Add new UUID FK columns to all child tables and populate via join
-- =============================================================================

-- ---- teams.id references ----

-- games.contender_id_a -> teams
ALTER TABLE games ADD COLUMN new_contender_id_a uuid;
UPDATE games SET new_contender_id_a = t.new_id FROM teams t WHERE games.contender_id_a = t.id;
ALTER TABLE games ALTER COLUMN new_contender_id_a SET NOT NULL;

-- games.contender_id_b -> teams
ALTER TABLE games ADD COLUMN new_contender_id_b uuid;
UPDATE games SET new_contender_id_b = t.new_id FROM teams t WHERE games.contender_id_b = t.id;
ALTER TABLE games ALTER COLUMN new_contender_id_b SET NOT NULL;

-- nfl_drives.possession_team_id -> teams
ALTER TABLE nfl_drives ADD COLUMN new_possession_team_id uuid;
UPDATE nfl_drives SET new_possession_team_id = t.new_id FROM teams t WHERE nfl_drives.possession_team_id = t.id;
ALTER TABLE nfl_drives ALTER COLUMN new_possession_team_id SET NOT NULL;

-- rosters.team_id -> teams (this is the PK after migration 1)
ALTER TABLE rosters ADD COLUMN new_team_id uuid;
UPDATE rosters SET new_team_id = t.new_id FROM teams t WHERE rosters.team_id = t.id;
ALTER TABLE rosters ALTER COLUMN new_team_id SET NOT NULL;

-- ---- individuals.id references ----

-- individual_statuses.individual_id -> individuals (this is the PK after migration 1)
ALTER TABLE individual_statuses ADD COLUMN new_individual_id uuid;
UPDATE individual_statuses SET new_individual_id = i.new_id FROM individuals i WHERE individual_statuses.individual_id = i.id;
ALTER TABLE individual_statuses ALTER COLUMN new_individual_id SET NOT NULL;

-- nfl_box_scores.individual_id -> individuals (part of composite PK)
ALTER TABLE nfl_box_scores ADD COLUMN new_individual_id uuid;
UPDATE nfl_box_scores SET new_individual_id = i.new_id FROM individuals i WHERE nfl_box_scores.individual_id = i.id;
ALTER TABLE nfl_box_scores ALTER COLUMN new_individual_id SET NOT NULL;

-- nba_box_scores.individual_id -> individuals (part of composite PK)
ALTER TABLE nba_box_scores ADD COLUMN new_individual_id uuid;
UPDATE nba_box_scores SET new_individual_id = i.new_id FROM individuals i WHERE nba_box_scores.individual_id = i.id;
ALTER TABLE nba_box_scores ALTER COLUMN new_individual_id SET NOT NULL;

-- nfl_markets.individual_id -> individuals
ALTER TABLE nfl_markets ADD COLUMN new_individual_id uuid;
UPDATE nfl_markets SET new_individual_id = i.new_id FROM individuals i WHERE nfl_markets.individual_id = i.id;
ALTER TABLE nfl_markets ALTER COLUMN new_individual_id SET NOT NULL;

-- nba_markets.individual_id -> individuals
ALTER TABLE nba_markets ADD COLUMN new_individual_id uuid;
UPDATE nba_markets SET new_individual_id = i.new_id FROM individuals i WHERE nba_markets.individual_id = i.id;
ALTER TABLE nba_markets ALTER COLUMN new_individual_id SET NOT NULL;

-- nfl_play_statistics.individual_id -> individuals
ALTER TABLE nfl_play_statistics ADD COLUMN new_individual_id uuid;
UPDATE nfl_play_statistics SET new_individual_id = i.new_id FROM individuals i WHERE nfl_play_statistics.individual_id = i.id;
ALTER TABLE nfl_play_statistics ALTER COLUMN new_individual_id SET NOT NULL;

-- nba_play_statistics.individual_id -> individuals
ALTER TABLE nba_play_statistics ADD COLUMN new_individual_id uuid;
UPDATE nba_play_statistics SET new_individual_id = i.new_id FROM individuals i WHERE nba_play_statistics.individual_id = i.id;
ALTER TABLE nba_play_statistics ALTER COLUMN new_individual_id SET NOT NULL;

-- rosters.individual_ids (bigint[] -> uuid[])
ALTER TABLE rosters ADD COLUMN new_individual_ids uuid[];
UPDATE rosters r SET new_individual_ids = (
    SELECT array_agg(i.new_id ORDER BY ordinality)
    FROM unnest(r.individual_ids) WITH ORDINALITY AS u(old_id, ordinality)
    JOIN individuals i ON i.id = u.old_id
);
ALTER TABLE rosters ALTER COLUMN new_individual_ids SET NOT NULL;

-- ---- games.id references ----

-- game_statuses.game_id -> games (this is the PK)
ALTER TABLE game_statuses ADD COLUMN new_game_id uuid;
UPDATE game_statuses SET new_game_id = g.new_id FROM games g WHERE game_statuses.game_id = g.id;
ALTER TABLE game_statuses ALTER COLUMN new_game_id SET NOT NULL;

-- nfl_drives.game_id -> games
ALTER TABLE nfl_drives ADD COLUMN new_game_id uuid;
UPDATE nfl_drives SET new_game_id = g.new_id FROM games g WHERE nfl_drives.game_id = g.id;
ALTER TABLE nfl_drives ALTER COLUMN new_game_id SET NOT NULL;

-- nba_plays.game_id -> games
ALTER TABLE nba_plays ADD COLUMN new_game_id uuid;
UPDATE nba_plays SET new_game_id = g.new_id FROM games g WHERE nba_plays.game_id = g.id;
ALTER TABLE nba_plays ALTER COLUMN new_game_id SET NOT NULL;

-- nfl_box_scores.game_id -> games (part of composite PK)
ALTER TABLE nfl_box_scores ADD COLUMN new_game_id uuid;
UPDATE nfl_box_scores SET new_game_id = g.new_id FROM games g WHERE nfl_box_scores.game_id = g.id;
ALTER TABLE nfl_box_scores ALTER COLUMN new_game_id SET NOT NULL;

-- nba_box_scores.game_id -> games (part of composite PK)
ALTER TABLE nba_box_scores ADD COLUMN new_game_id uuid;
UPDATE nba_box_scores SET new_game_id = g.new_id FROM games g WHERE nba_box_scores.game_id = g.id;
ALTER TABLE nba_box_scores ALTER COLUMN new_game_id SET NOT NULL;

-- nfl_markets.game_id -> games
ALTER TABLE nfl_markets ADD COLUMN new_game_id uuid;
UPDATE nfl_markets SET new_game_id = g.new_id FROM games g WHERE nfl_markets.game_id = g.id;
ALTER TABLE nfl_markets ALTER COLUMN new_game_id SET NOT NULL;

-- nba_markets.game_id -> games
ALTER TABLE nba_markets ADD COLUMN new_game_id uuid;
UPDATE nba_markets SET new_game_id = g.new_id FROM games g WHERE nba_markets.game_id = g.id;
ALTER TABLE nba_markets ALTER COLUMN new_game_id SET NOT NULL;

-- ---- Polymorphic: entity_vendor_ids.entity_id -> games/individuals/teams ----
ALTER TABLE entity_vendor_ids ADD COLUMN new_entity_id uuid;
UPDATE entity_vendor_ids SET new_entity_id = g.new_id FROM games g WHERE entity_vendor_ids.entity_id = g.id AND entity_vendor_ids.entity_type = 'game';
UPDATE entity_vendor_ids SET new_entity_id = i.new_id FROM individuals i WHERE entity_vendor_ids.entity_id = i.id AND entity_vendor_ids.entity_type = 'individual';
UPDATE entity_vendor_ids SET new_entity_id = t.new_id FROM teams t WHERE entity_vendor_ids.entity_id = t.id AND entity_vendor_ids.entity_type = 'team';
ALTER TABLE entity_vendor_ids ALTER COLUMN new_entity_id SET NOT NULL;

-- ---- Polymorphic: odds_blaze_market_ids.entity_id -> nba_markets/nfl_markets ----
ALTER TABLE odds_blaze_market_ids ADD COLUMN new_entity_id uuid;
UPDATE odds_blaze_market_ids SET new_entity_id = m.new_id FROM nba_markets m WHERE odds_blaze_market_ids.entity_id = m.id AND odds_blaze_market_ids.entity_type = 'nba_market';
UPDATE odds_blaze_market_ids SET new_entity_id = m.new_id FROM nfl_markets m WHERE odds_blaze_market_ids.entity_id = m.id AND odds_blaze_market_ids.entity_type = 'nfl_market';
ALTER TABLE odds_blaze_market_ids ALTER COLUMN new_entity_id SET NOT NULL;

-- =============================================================================
-- PHASE 3: Drop all old FK constraints
-- =============================================================================
ALTER TABLE nfl_drives DROP CONSTRAINT nfl_drives_game_id_fkey;
ALTER TABLE nfl_drives DROP CONSTRAINT nfl_drives_possession_team_id_fkey;
ALTER TABLE nba_plays DROP CONSTRAINT nba_plays_game_id_fkey;
ALTER TABLE game_statuses DROP CONSTRAINT game_statuses_game_id_fkey;
ALTER TABLE nfl_box_scores DROP CONSTRAINT nfl_box_scores_game_id_fkey;
ALTER TABLE nfl_box_scores DROP CONSTRAINT nfl_box_scores_individual_id_fkey;
ALTER TABLE nba_box_scores DROP CONSTRAINT nba_box_scores_game_id_fkey;
ALTER TABLE nba_box_scores DROP CONSTRAINT nba_box_scores_individual_id_fkey;
ALTER TABLE nfl_markets DROP CONSTRAINT nfl_markets_game_id_fkey;
ALTER TABLE nfl_markets DROP CONSTRAINT nfl_markets_individual_id_fkey;
ALTER TABLE nba_markets DROP CONSTRAINT nba_markets_game_id_fkey;
ALTER TABLE nba_markets DROP CONSTRAINT nba_markets_individual_id_fkey;
ALTER TABLE nfl_play_statistics DROP CONSTRAINT nfl_play_statistics_individual_id_fkey;
ALTER TABLE nba_play_statistics DROP CONSTRAINT nba_play_statistics_individual_id_fkey;

-- =============================================================================
-- PHASE 4: Drop old PK/unique constraints, indexes, and columns
-- =============================================================================

-- ---- teams ----
ALTER TABLE teams DROP CONSTRAINT teams_pkey;
ALTER TABLE teams DROP CONSTRAINT teams_new_id_unique;
ALTER TABLE teams DROP COLUMN id;
ALTER TABLE teams RENAME COLUMN new_id TO id;
ALTER TABLE teams ADD CONSTRAINT teams_pkey PRIMARY KEY (id);
ALTER TABLE teams ALTER COLUMN id SET DEFAULT gen_random_uuid();

-- ---- individuals ----
ALTER TABLE individuals DROP CONSTRAINT players_pkey;
ALTER TABLE individuals DROP CONSTRAINT individuals_new_id_unique;
ALTER TABLE individuals DROP COLUMN id;
ALTER TABLE individuals RENAME COLUMN new_id TO id;
ALTER TABLE individuals ADD CONSTRAINT individuals_pkey PRIMARY KEY (id);
ALTER TABLE individuals ALTER COLUMN id SET DEFAULT gen_random_uuid();

-- ---- games ----
ALTER TABLE games DROP CONSTRAINT games_pkey;
ALTER TABLE games DROP CONSTRAINT games_new_id_unique;
ALTER TABLE games DROP COLUMN id;
ALTER TABLE games RENAME COLUMN new_id TO id;
ALTER TABLE games ADD CONSTRAINT games_pkey PRIMARY KEY (id);
ALTER TABLE games ALTER COLUMN id SET DEFAULT gen_random_uuid();

-- games: swap contender columns
ALTER TABLE games DROP COLUMN contender_id_a;
ALTER TABLE games RENAME COLUMN new_contender_id_a TO contender_id_a;
ALTER TABLE games DROP COLUMN contender_id_b;
ALTER TABLE games RENAME COLUMN new_contender_id_b TO contender_id_b;

-- ---- nfl_markets ----
ALTER TABLE nfl_markets DROP CONSTRAINT nfl_markets_pkey;
ALTER TABLE nfl_markets DROP CONSTRAINT nfl_markets_new_id_unique;
DROP INDEX IF EXISTS idx_nfl_markets_game_id;
DROP INDEX IF EXISTS idx_nfl_markets_individual_id;
DROP INDEX IF EXISTS idx_nfl_markets_game_individual;
ALTER TABLE nfl_markets DROP CONSTRAINT nfl_markets_game_individual_type_line_unique;
ALTER TABLE nfl_markets DROP COLUMN id;
ALTER TABLE nfl_markets RENAME COLUMN new_id TO id;
ALTER TABLE nfl_markets DROP COLUMN game_id;
ALTER TABLE nfl_markets RENAME COLUMN new_game_id TO game_id;
ALTER TABLE nfl_markets DROP COLUMN individual_id;
ALTER TABLE nfl_markets RENAME COLUMN new_individual_id TO individual_id;
ALTER TABLE nfl_markets ADD CONSTRAINT nfl_markets_pkey PRIMARY KEY (id);
ALTER TABLE nfl_markets ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE nfl_markets ADD CONSTRAINT nfl_markets_game_individual_type_line_unique UNIQUE (game_id, individual_id, market_type, market_line);
CREATE INDEX idx_nfl_markets_game_id ON nfl_markets (game_id);
CREATE INDEX idx_nfl_markets_individual_id ON nfl_markets (individual_id);
CREATE INDEX idx_nfl_markets_game_individual ON nfl_markets (game_id, individual_id);
ALTER TABLE nfl_markets ADD CONSTRAINT nfl_markets_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id);
ALTER TABLE nfl_markets ADD CONSTRAINT nfl_markets_individual_id_fkey FOREIGN KEY (individual_id) REFERENCES individuals(id);

-- ---- nba_markets ----
ALTER TABLE nba_markets DROP CONSTRAINT nba_markets_pkey;
ALTER TABLE nba_markets DROP CONSTRAINT nba_markets_new_id_unique;
DROP INDEX IF EXISTS idx_nba_markets_game_id;
DROP INDEX IF EXISTS idx_nba_markets_individual_id;
DROP INDEX IF EXISTS idx_nba_markets_game_individual;
ALTER TABLE nba_markets DROP CONSTRAINT nba_markets_game_individual_type_line_unique;
ALTER TABLE nba_markets DROP COLUMN id;
ALTER TABLE nba_markets RENAME COLUMN new_id TO id;
ALTER TABLE nba_markets DROP COLUMN game_id;
ALTER TABLE nba_markets RENAME COLUMN new_game_id TO game_id;
ALTER TABLE nba_markets DROP COLUMN individual_id;
ALTER TABLE nba_markets RENAME COLUMN new_individual_id TO individual_id;
ALTER TABLE nba_markets ADD CONSTRAINT nba_markets_pkey PRIMARY KEY (id);
ALTER TABLE nba_markets ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE nba_markets ADD CONSTRAINT nba_markets_game_individual_type_line_unique UNIQUE (game_id, individual_id, market_type, market_line);
CREATE INDEX idx_nba_markets_game_id ON nba_markets (game_id);
CREATE INDEX idx_nba_markets_individual_id ON nba_markets (individual_id);
CREATE INDEX idx_nba_markets_game_individual ON nba_markets (game_id, individual_id);
ALTER TABLE nba_markets ADD CONSTRAINT nba_markets_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id);
ALTER TABLE nba_markets ADD CONSTRAINT nba_markets_individual_id_fkey FOREIGN KEY (individual_id) REFERENCES individuals(id);

-- =============================================================================
-- PHASE 5: Swap old/new FK columns in remaining child tables
-- =============================================================================

-- ---- nfl_drives ----
DROP INDEX IF EXISTS index_nfl_drives_on_game_id;
ALTER TABLE nfl_drives DROP CONSTRAINT nfl_drives_game_id_vendor_id_key;
ALTER TABLE nfl_drives DROP COLUMN game_id;
ALTER TABLE nfl_drives RENAME COLUMN new_game_id TO game_id;
ALTER TABLE nfl_drives DROP COLUMN possession_team_id;
ALTER TABLE nfl_drives RENAME COLUMN new_possession_team_id TO possession_team_id;
ALTER TABLE nfl_drives ADD CONSTRAINT nfl_drives_game_id_vendor_id_key UNIQUE (game_id, sportradar_id);
CREATE INDEX index_nfl_drives_on_game_id ON nfl_drives (game_id);
ALTER TABLE nfl_drives ADD CONSTRAINT nfl_drives_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id);
ALTER TABLE nfl_drives ADD CONSTRAINT nfl_drives_possession_team_id_fkey FOREIGN KEY (possession_team_id) REFERENCES teams(id);

-- ---- nba_plays ----
DROP INDEX IF EXISTS idx_nba_plays_game_id;
ALTER TABLE nba_plays DROP CONSTRAINT nba_plays_game_vendor_unique;
ALTER TABLE nba_plays DROP COLUMN game_id;
ALTER TABLE nba_plays RENAME COLUMN new_game_id TO game_id;
ALTER TABLE nba_plays ADD CONSTRAINT nba_plays_game_vendor_unique UNIQUE (game_id, sportradar_id);
CREATE INDEX idx_nba_plays_game_id ON nba_plays (game_id);
ALTER TABLE nba_plays ADD CONSTRAINT nba_plays_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id);

-- ---- game_statuses ----
ALTER TABLE game_statuses DROP CONSTRAINT game_statuses_pkey;
ALTER TABLE game_statuses DROP COLUMN game_id;
ALTER TABLE game_statuses RENAME COLUMN new_game_id TO game_id;
ALTER TABLE game_statuses ADD CONSTRAINT game_statuses_pkey PRIMARY KEY (game_id);
ALTER TABLE game_statuses ADD CONSTRAINT game_statuses_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id);

-- ---- nfl_box_scores ----
ALTER TABLE nfl_box_scores DROP CONSTRAINT nfl_box_scores_pkey;
DROP INDEX IF EXISTS index_nfl_box_scores_on_game_id;
ALTER TABLE nfl_box_scores DROP COLUMN game_id;
ALTER TABLE nfl_box_scores RENAME COLUMN new_game_id TO game_id;
ALTER TABLE nfl_box_scores DROP COLUMN individual_id;
ALTER TABLE nfl_box_scores RENAME COLUMN new_individual_id TO individual_id;
ALTER TABLE nfl_box_scores ADD CONSTRAINT nfl_box_scores_pkey PRIMARY KEY (game_id, individual_id);
CREATE INDEX index_nfl_box_scores_on_game_id ON nfl_box_scores (game_id);
ALTER TABLE nfl_box_scores ADD CONSTRAINT nfl_box_scores_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id);
ALTER TABLE nfl_box_scores ADD CONSTRAINT nfl_box_scores_individual_id_fkey FOREIGN KEY (individual_id) REFERENCES individuals(id);

-- ---- nba_box_scores ----
ALTER TABLE nba_box_scores DROP CONSTRAINT nba_box_scores_pkey;
DROP INDEX IF EXISTS index_nba_box_scores_on_game_id;
ALTER TABLE nba_box_scores DROP COLUMN game_id;
ALTER TABLE nba_box_scores RENAME COLUMN new_game_id TO game_id;
ALTER TABLE nba_box_scores DROP COLUMN individual_id;
ALTER TABLE nba_box_scores RENAME COLUMN new_individual_id TO individual_id;
ALTER TABLE nba_box_scores ADD CONSTRAINT nba_box_scores_pkey PRIMARY KEY (game_id, individual_id);
CREATE INDEX index_nba_box_scores_on_game_id ON nba_box_scores (game_id);
ALTER TABLE nba_box_scores ADD CONSTRAINT nba_box_scores_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id);
ALTER TABLE nba_box_scores ADD CONSTRAINT nba_box_scores_individual_id_fkey FOREIGN KEY (individual_id) REFERENCES individuals(id);

-- ---- individual_statuses ----
ALTER TABLE individual_statuses DROP CONSTRAINT individual_statuses_pkey;
ALTER TABLE individual_statuses DROP COLUMN individual_id;
ALTER TABLE individual_statuses RENAME COLUMN new_individual_id TO individual_id;
ALTER TABLE individual_statuses ADD CONSTRAINT individual_statuses_pkey PRIMARY KEY (individual_id);

-- ---- nfl_play_statistics ----
DROP INDEX IF EXISTS index_nfl_play_statistics_on_individual_id;
ALTER TABLE nfl_play_statistics DROP COLUMN individual_id;
ALTER TABLE nfl_play_statistics RENAME COLUMN new_individual_id TO individual_id;
CREATE INDEX index_nfl_play_statistics_on_individual_id ON nfl_play_statistics (individual_id);
ALTER TABLE nfl_play_statistics ADD CONSTRAINT nfl_play_statistics_individual_id_fkey FOREIGN KEY (individual_id) REFERENCES individuals(id);

-- ---- nba_play_statistics ----
ALTER TABLE nba_play_statistics DROP COLUMN individual_id;
ALTER TABLE nba_play_statistics RENAME COLUMN new_individual_id TO individual_id;
ALTER TABLE nba_play_statistics ADD CONSTRAINT nba_play_statistics_individual_id_fkey FOREIGN KEY (individual_id) REFERENCES individuals(id);

-- ---- rosters ----
ALTER TABLE rosters DROP CONSTRAINT rosters_pkey;
ALTER TABLE rosters DROP COLUMN team_id;
ALTER TABLE rosters RENAME COLUMN new_team_id TO team_id;
ALTER TABLE rosters DROP COLUMN individual_ids;
ALTER TABLE rosters RENAME COLUMN new_individual_ids TO individual_ids;
ALTER TABLE rosters ADD CONSTRAINT rosters_pkey PRIMARY KEY (team_id);

-- ---- entity_vendor_ids (polymorphic) ----
ALTER TABLE entity_vendor_ids DROP CONSTRAINT entity_vendor_ids_pkey;
ALTER TABLE entity_vendor_ids DROP CONSTRAINT entity_vendor_ids_type_vendor_id_unique;
DROP INDEX IF EXISTS idx_entity_vendor_ids_type_entity;
ALTER TABLE entity_vendor_ids DROP COLUMN entity_id;
ALTER TABLE entity_vendor_ids RENAME COLUMN new_entity_id TO entity_id;
ALTER TABLE entity_vendor_ids ADD CONSTRAINT entity_vendor_ids_pkey PRIMARY KEY (entity_type, entity_id, vendor);
ALTER TABLE entity_vendor_ids ADD CONSTRAINT entity_vendor_ids_type_vendor_id_unique UNIQUE (entity_type, vendor, vendor_id);
CREATE INDEX idx_entity_vendor_ids_type_entity ON entity_vendor_ids (entity_type, entity_id);

-- ---- odds_blaze_market_ids (polymorphic) ----
ALTER TABLE odds_blaze_market_ids DROP CONSTRAINT odds_blaze_market_ids_entity_sportsbook_side_unique;
ALTER TABLE odds_blaze_market_ids DROP CONSTRAINT odds_blaze_market_ids_type_sportsbook_id_unique;
ALTER TABLE odds_blaze_market_ids DROP COLUMN entity_id;
ALTER TABLE odds_blaze_market_ids RENAME COLUMN new_entity_id TO entity_id;
ALTER TABLE odds_blaze_market_ids ADD CONSTRAINT odds_blaze_market_ids_entity_sportsbook_side_unique UNIQUE (entity_type, entity_id, sportsbook, side);
ALTER TABLE odds_blaze_market_ids ADD CONSTRAINT odds_blaze_market_ids_type_sportsbook_id_unique UNIQUE (entity_type, sportsbook, odds_blaze_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- =============================================================================
-- NOTE: The down migration generates new sequential integer IDs. The original
-- integer ID values are NOT preserved, but all FK relationships are maintained.
-- =============================================================================

-- PHASE 1: Add integer ID columns to the 5 parent tables
ALTER TABLE teams ADD COLUMN old_id serial;
ALTER TABLE individuals ADD COLUMN old_id serial;
ALTER TABLE games ADD COLUMN old_id serial;
ALTER TABLE nfl_markets ADD COLUMN old_id serial;
ALTER TABLE nba_markets ADD COLUMN old_id serial;

-- PHASE 2: Add integer FK columns to child tables, populate via join

-- ---- teams references ----
ALTER TABLE games ADD COLUMN old_contender_id_a bigint;
UPDATE games SET old_contender_id_a = t.old_id FROM teams t WHERE games.contender_id_a = t.id;
ALTER TABLE games ALTER COLUMN old_contender_id_a SET NOT NULL;

ALTER TABLE games ADD COLUMN old_contender_id_b bigint;
UPDATE games SET old_contender_id_b = t.old_id FROM teams t WHERE games.contender_id_b = t.id;
ALTER TABLE games ALTER COLUMN old_contender_id_b SET NOT NULL;

ALTER TABLE nfl_drives ADD COLUMN old_possession_team_id integer;
UPDATE nfl_drives SET old_possession_team_id = t.old_id FROM teams t WHERE nfl_drives.possession_team_id = t.id;
ALTER TABLE nfl_drives ALTER COLUMN old_possession_team_id SET NOT NULL;

ALTER TABLE rosters ADD COLUMN old_team_id bigint;
UPDATE rosters SET old_team_id = t.old_id FROM teams t WHERE rosters.team_id = t.id;
ALTER TABLE rosters ALTER COLUMN old_team_id SET NOT NULL;

-- ---- individuals references ----
ALTER TABLE individual_statuses ADD COLUMN old_individual_id bigint;
UPDATE individual_statuses SET old_individual_id = i.old_id FROM individuals i WHERE individual_statuses.individual_id = i.id;
ALTER TABLE individual_statuses ALTER COLUMN old_individual_id SET NOT NULL;

ALTER TABLE nfl_box_scores ADD COLUMN old_individual_id integer;
UPDATE nfl_box_scores SET old_individual_id = i.old_id FROM individuals i WHERE nfl_box_scores.individual_id = i.id;
ALTER TABLE nfl_box_scores ALTER COLUMN old_individual_id SET NOT NULL;

ALTER TABLE nba_box_scores ADD COLUMN old_individual_id integer;
UPDATE nba_box_scores SET old_individual_id = i.old_id FROM individuals i WHERE nba_box_scores.individual_id = i.id;
ALTER TABLE nba_box_scores ALTER COLUMN old_individual_id SET NOT NULL;

ALTER TABLE nfl_markets ADD COLUMN old_individual_id integer;
UPDATE nfl_markets SET old_individual_id = i.old_id FROM individuals i WHERE nfl_markets.individual_id = i.id;
ALTER TABLE nfl_markets ALTER COLUMN old_individual_id SET NOT NULL;

ALTER TABLE nba_markets ADD COLUMN old_individual_id integer;
UPDATE nba_markets SET old_individual_id = i.old_id FROM individuals i WHERE nba_markets.individual_id = i.id;
ALTER TABLE nba_markets ALTER COLUMN old_individual_id SET NOT NULL;

ALTER TABLE nfl_play_statistics ADD COLUMN old_individual_id integer;
UPDATE nfl_play_statistics SET old_individual_id = i.old_id FROM individuals i WHERE nfl_play_statistics.individual_id = i.id;
ALTER TABLE nfl_play_statistics ALTER COLUMN old_individual_id SET NOT NULL;

ALTER TABLE nba_play_statistics ADD COLUMN old_individual_id integer;
UPDATE nba_play_statistics SET old_individual_id = i.old_id FROM individuals i WHERE nba_play_statistics.individual_id = i.id;
ALTER TABLE nba_play_statistics ALTER COLUMN old_individual_id SET NOT NULL;

ALTER TABLE rosters ADD COLUMN old_individual_ids bigint[];
UPDATE rosters r SET old_individual_ids = (
    SELECT array_agg(i.old_id ORDER BY ordinality)
    FROM unnest(r.individual_ids) WITH ORDINALITY AS u(old_uuid, ordinality)
    JOIN individuals i ON i.id = u.old_uuid
);
ALTER TABLE rosters ALTER COLUMN old_individual_ids SET NOT NULL;

-- ---- games references ----
ALTER TABLE game_statuses ADD COLUMN old_game_id integer;
UPDATE game_statuses SET old_game_id = g.old_id FROM games g WHERE game_statuses.game_id = g.id;
ALTER TABLE game_statuses ALTER COLUMN old_game_id SET NOT NULL;

ALTER TABLE nfl_drives ADD COLUMN old_game_id integer;
UPDATE nfl_drives SET old_game_id = g.old_id FROM games g WHERE nfl_drives.game_id = g.id;
ALTER TABLE nfl_drives ALTER COLUMN old_game_id SET NOT NULL;

ALTER TABLE nba_plays ADD COLUMN old_game_id integer;
UPDATE nba_plays SET old_game_id = g.old_id FROM games g WHERE nba_plays.game_id = g.id;
ALTER TABLE nba_plays ALTER COLUMN old_game_id SET NOT NULL;

ALTER TABLE nfl_box_scores ADD COLUMN old_game_id integer;
UPDATE nfl_box_scores SET old_game_id = g.old_id FROM games g WHERE nfl_box_scores.game_id = g.id;
ALTER TABLE nfl_box_scores ALTER COLUMN old_game_id SET NOT NULL;

ALTER TABLE nba_box_scores ADD COLUMN old_game_id integer;
UPDATE nba_box_scores SET old_game_id = g.old_id FROM games g WHERE nba_box_scores.game_id = g.id;
ALTER TABLE nba_box_scores ALTER COLUMN old_game_id SET NOT NULL;

ALTER TABLE nfl_markets ADD COLUMN old_game_id integer;
UPDATE nfl_markets SET old_game_id = g.old_id FROM games g WHERE nfl_markets.game_id = g.id;
ALTER TABLE nfl_markets ALTER COLUMN old_game_id SET NOT NULL;

ALTER TABLE nba_markets ADD COLUMN old_game_id integer;
UPDATE nba_markets SET old_game_id = g.old_id FROM games g WHERE nba_markets.game_id = g.id;
ALTER TABLE nba_markets ALTER COLUMN old_game_id SET NOT NULL;

-- ---- polymorphic: entity_vendor_ids ----
ALTER TABLE entity_vendor_ids ADD COLUMN old_entity_id integer;
UPDATE entity_vendor_ids SET old_entity_id = g.old_id FROM games g WHERE entity_vendor_ids.entity_id = g.id AND entity_vendor_ids.entity_type = 'game';
UPDATE entity_vendor_ids SET old_entity_id = i.old_id FROM individuals i WHERE entity_vendor_ids.entity_id = i.id AND entity_vendor_ids.entity_type = 'individual';
UPDATE entity_vendor_ids SET old_entity_id = t.old_id FROM teams t WHERE entity_vendor_ids.entity_id = t.id AND entity_vendor_ids.entity_type = 'team';
ALTER TABLE entity_vendor_ids ALTER COLUMN old_entity_id SET NOT NULL;

-- ---- polymorphic: odds_blaze_market_ids ----
ALTER TABLE odds_blaze_market_ids ADD COLUMN old_entity_id integer;
UPDATE odds_blaze_market_ids SET old_entity_id = m.old_id FROM nba_markets m WHERE odds_blaze_market_ids.entity_id = m.id AND odds_blaze_market_ids.entity_type = 'nba_market';
UPDATE odds_blaze_market_ids SET old_entity_id = m.old_id FROM nfl_markets m WHERE odds_blaze_market_ids.entity_id = m.id AND odds_blaze_market_ids.entity_type = 'nfl_market';
ALTER TABLE odds_blaze_market_ids ALTER COLUMN old_entity_id SET NOT NULL;

-- PHASE 3: Drop FK constraints
ALTER TABLE nfl_drives DROP CONSTRAINT nfl_drives_game_id_fkey;
ALTER TABLE nfl_drives DROP CONSTRAINT nfl_drives_possession_team_id_fkey;
ALTER TABLE nba_plays DROP CONSTRAINT nba_plays_game_id_fkey;
ALTER TABLE game_statuses DROP CONSTRAINT game_statuses_game_id_fkey;
ALTER TABLE nfl_box_scores DROP CONSTRAINT nfl_box_scores_game_id_fkey;
ALTER TABLE nfl_box_scores DROP CONSTRAINT nfl_box_scores_individual_id_fkey;
ALTER TABLE nba_box_scores DROP CONSTRAINT nba_box_scores_game_id_fkey;
ALTER TABLE nba_box_scores DROP CONSTRAINT nba_box_scores_individual_id_fkey;
ALTER TABLE nfl_markets DROP CONSTRAINT nfl_markets_game_id_fkey;
ALTER TABLE nfl_markets DROP CONSTRAINT nfl_markets_individual_id_fkey;
ALTER TABLE nba_markets DROP CONSTRAINT nba_markets_game_id_fkey;
ALTER TABLE nba_markets DROP CONSTRAINT nba_markets_individual_id_fkey;
ALTER TABLE nfl_play_statistics DROP CONSTRAINT nfl_play_statistics_individual_id_fkey;
ALTER TABLE nba_play_statistics DROP CONSTRAINT nba_play_statistics_individual_id_fkey;

-- PHASE 4: Swap columns on parent tables

-- ---- teams ----
ALTER TABLE teams DROP CONSTRAINT teams_pkey;
ALTER TABLE teams DROP COLUMN id;
ALTER TABLE teams RENAME COLUMN old_id TO id;
ALTER TABLE teams ADD CONSTRAINT teams_pkey PRIMARY KEY (id);

-- ---- individuals ----
ALTER TABLE individuals DROP CONSTRAINT individuals_pkey;
ALTER TABLE individuals DROP COLUMN id;
ALTER TABLE individuals RENAME COLUMN old_id TO id;
ALTER TABLE individuals ADD CONSTRAINT players_pkey PRIMARY KEY (id);

-- ---- games ----
ALTER TABLE games DROP CONSTRAINT games_pkey;
ALTER TABLE games DROP COLUMN id;
ALTER TABLE games RENAME COLUMN old_id TO id;
ALTER TABLE games ADD CONSTRAINT games_pkey PRIMARY KEY (id);

ALTER TABLE games DROP COLUMN contender_id_a;
ALTER TABLE games RENAME COLUMN old_contender_id_a TO contender_id_a;
ALTER TABLE games DROP COLUMN contender_id_b;
ALTER TABLE games RENAME COLUMN old_contender_id_b TO contender_id_b;

-- ---- nfl_markets ----
ALTER TABLE nfl_markets DROP CONSTRAINT nfl_markets_pkey;
DROP INDEX IF EXISTS idx_nfl_markets_game_id;
DROP INDEX IF EXISTS idx_nfl_markets_individual_id;
DROP INDEX IF EXISTS idx_nfl_markets_game_individual;
ALTER TABLE nfl_markets DROP CONSTRAINT nfl_markets_game_individual_type_line_unique;
ALTER TABLE nfl_markets DROP COLUMN id;
ALTER TABLE nfl_markets RENAME COLUMN old_id TO id;
ALTER TABLE nfl_markets DROP COLUMN game_id;
ALTER TABLE nfl_markets RENAME COLUMN old_game_id TO game_id;
ALTER TABLE nfl_markets DROP COLUMN individual_id;
ALTER TABLE nfl_markets RENAME COLUMN old_individual_id TO individual_id;
ALTER TABLE nfl_markets ADD CONSTRAINT nfl_markets_pkey PRIMARY KEY (id);
ALTER TABLE nfl_markets ADD CONSTRAINT nfl_markets_game_individual_type_line_unique UNIQUE (game_id, individual_id, market_type, market_line);
CREATE INDEX idx_nfl_markets_game_id ON nfl_markets (game_id);
CREATE INDEX idx_nfl_markets_individual_id ON nfl_markets (individual_id);
CREATE INDEX idx_nfl_markets_game_individual ON nfl_markets (game_id, individual_id);
ALTER TABLE nfl_markets ADD CONSTRAINT nfl_markets_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id);
ALTER TABLE nfl_markets ADD CONSTRAINT nfl_markets_individual_id_fkey FOREIGN KEY (individual_id) REFERENCES individuals(id);

-- ---- nba_markets ----
ALTER TABLE nba_markets DROP CONSTRAINT nba_markets_pkey;
DROP INDEX IF EXISTS idx_nba_markets_game_id;
DROP INDEX IF EXISTS idx_nba_markets_individual_id;
DROP INDEX IF EXISTS idx_nba_markets_game_individual;
ALTER TABLE nba_markets DROP CONSTRAINT nba_markets_game_individual_type_line_unique;
ALTER TABLE nba_markets DROP COLUMN id;
ALTER TABLE nba_markets RENAME COLUMN old_id TO id;
ALTER TABLE nba_markets DROP COLUMN game_id;
ALTER TABLE nba_markets RENAME COLUMN old_game_id TO game_id;
ALTER TABLE nba_markets DROP COLUMN individual_id;
ALTER TABLE nba_markets RENAME COLUMN old_individual_id TO individual_id;
ALTER TABLE nba_markets ADD CONSTRAINT nba_markets_pkey PRIMARY KEY (id);
ALTER TABLE nba_markets ADD CONSTRAINT nba_markets_game_individual_type_line_unique UNIQUE (game_id, individual_id, market_type, market_line);
CREATE INDEX idx_nba_markets_game_id ON nba_markets (game_id);
CREATE INDEX idx_nba_markets_individual_id ON nba_markets (individual_id);
CREATE INDEX idx_nba_markets_game_individual ON nba_markets (game_id, individual_id);
ALTER TABLE nba_markets ADD CONSTRAINT nba_markets_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id);
ALTER TABLE nba_markets ADD CONSTRAINT nba_markets_individual_id_fkey FOREIGN KEY (individual_id) REFERENCES individuals(id);

-- PHASE 5: Swap FK columns in remaining child tables

-- ---- nfl_drives ----
DROP INDEX IF EXISTS index_nfl_drives_on_game_id;
ALTER TABLE nfl_drives DROP CONSTRAINT nfl_drives_game_id_vendor_id_key;
ALTER TABLE nfl_drives DROP COLUMN game_id;
ALTER TABLE nfl_drives RENAME COLUMN old_game_id TO game_id;
ALTER TABLE nfl_drives DROP COLUMN possession_team_id;
ALTER TABLE nfl_drives RENAME COLUMN old_possession_team_id TO possession_team_id;
ALTER TABLE nfl_drives ADD CONSTRAINT nfl_drives_game_id_vendor_id_key UNIQUE (game_id, sportradar_id);
CREATE INDEX index_nfl_drives_on_game_id ON nfl_drives (game_id);
ALTER TABLE nfl_drives ADD CONSTRAINT nfl_drives_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id);
ALTER TABLE nfl_drives ADD CONSTRAINT nfl_drives_possession_team_id_fkey FOREIGN KEY (possession_team_id) REFERENCES teams(id);

-- ---- nba_plays ----
DROP INDEX IF EXISTS idx_nba_plays_game_id;
ALTER TABLE nba_plays DROP CONSTRAINT nba_plays_game_vendor_unique;
ALTER TABLE nba_plays DROP COLUMN game_id;
ALTER TABLE nba_plays RENAME COLUMN old_game_id TO game_id;
ALTER TABLE nba_plays ADD CONSTRAINT nba_plays_game_vendor_unique UNIQUE (game_id, sportradar_id);
CREATE INDEX idx_nba_plays_game_id ON nba_plays (game_id);
ALTER TABLE nba_plays ADD CONSTRAINT nba_plays_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id);

-- ---- game_statuses ----
ALTER TABLE game_statuses DROP CONSTRAINT game_statuses_pkey;
ALTER TABLE game_statuses DROP COLUMN game_id;
ALTER TABLE game_statuses RENAME COLUMN old_game_id TO game_id;
ALTER TABLE game_statuses ADD CONSTRAINT game_statuses_pkey PRIMARY KEY (game_id);
ALTER TABLE game_statuses ADD CONSTRAINT game_statuses_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id);

-- ---- nfl_box_scores ----
ALTER TABLE nfl_box_scores DROP CONSTRAINT nfl_box_scores_pkey;
DROP INDEX IF EXISTS index_nfl_box_scores_on_game_id;
ALTER TABLE nfl_box_scores DROP COLUMN game_id;
ALTER TABLE nfl_box_scores RENAME COLUMN old_game_id TO game_id;
ALTER TABLE nfl_box_scores DROP COLUMN individual_id;
ALTER TABLE nfl_box_scores RENAME COLUMN old_individual_id TO individual_id;
ALTER TABLE nfl_box_scores ADD CONSTRAINT nfl_box_scores_pkey PRIMARY KEY (game_id, individual_id);
CREATE INDEX index_nfl_box_scores_on_game_id ON nfl_box_scores (game_id);
ALTER TABLE nfl_box_scores ADD CONSTRAINT nfl_box_scores_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id);
ALTER TABLE nfl_box_scores ADD CONSTRAINT nfl_box_scores_individual_id_fkey FOREIGN KEY (individual_id) REFERENCES individuals(id);

-- ---- nba_box_scores ----
ALTER TABLE nba_box_scores DROP CONSTRAINT nba_box_scores_pkey;
DROP INDEX IF EXISTS index_nba_box_scores_on_game_id;
ALTER TABLE nba_box_scores DROP COLUMN game_id;
ALTER TABLE nba_box_scores RENAME COLUMN old_game_id TO game_id;
ALTER TABLE nba_box_scores DROP COLUMN individual_id;
ALTER TABLE nba_box_scores RENAME COLUMN old_individual_id TO individual_id;
ALTER TABLE nba_box_scores ADD CONSTRAINT nba_box_scores_pkey PRIMARY KEY (game_id, individual_id);
CREATE INDEX index_nba_box_scores_on_game_id ON nba_box_scores (game_id);
ALTER TABLE nba_box_scores ADD CONSTRAINT nba_box_scores_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id);
ALTER TABLE nba_box_scores ADD CONSTRAINT nba_box_scores_individual_id_fkey FOREIGN KEY (individual_id) REFERENCES individuals(id);

-- ---- individual_statuses ----
ALTER TABLE individual_statuses DROP CONSTRAINT individual_statuses_pkey;
ALTER TABLE individual_statuses DROP COLUMN individual_id;
ALTER TABLE individual_statuses RENAME COLUMN old_individual_id TO individual_id;
ALTER TABLE individual_statuses ADD CONSTRAINT individual_statuses_pkey PRIMARY KEY (individual_id);

-- ---- nfl_play_statistics ----
DROP INDEX IF EXISTS index_nfl_play_statistics_on_individual_id;
ALTER TABLE nfl_play_statistics DROP COLUMN individual_id;
ALTER TABLE nfl_play_statistics RENAME COLUMN old_individual_id TO individual_id;
CREATE INDEX index_nfl_play_statistics_on_individual_id ON nfl_play_statistics (individual_id);
ALTER TABLE nfl_play_statistics ADD CONSTRAINT nfl_play_statistics_individual_id_fkey FOREIGN KEY (individual_id) REFERENCES individuals(id);

-- ---- nba_play_statistics ----
ALTER TABLE nba_play_statistics DROP COLUMN individual_id;
ALTER TABLE nba_play_statistics RENAME COLUMN old_individual_id TO individual_id;
ALTER TABLE nba_play_statistics ADD CONSTRAINT nba_play_statistics_individual_id_fkey FOREIGN KEY (individual_id) REFERENCES individuals(id);

-- ---- rosters ----
ALTER TABLE rosters DROP CONSTRAINT rosters_pkey;
ALTER TABLE rosters DROP COLUMN team_id;
ALTER TABLE rosters RENAME COLUMN old_team_id TO team_id;
ALTER TABLE rosters DROP COLUMN individual_ids;
ALTER TABLE rosters RENAME COLUMN old_individual_ids TO individual_ids;
ALTER TABLE rosters ADD CONSTRAINT rosters_pkey PRIMARY KEY (team_id);

-- ---- entity_vendor_ids ----
ALTER TABLE entity_vendor_ids DROP CONSTRAINT entity_vendor_ids_pkey;
ALTER TABLE entity_vendor_ids DROP CONSTRAINT entity_vendor_ids_type_vendor_id_unique;
DROP INDEX IF EXISTS idx_entity_vendor_ids_type_entity;
ALTER TABLE entity_vendor_ids DROP COLUMN entity_id;
ALTER TABLE entity_vendor_ids RENAME COLUMN old_entity_id TO entity_id;
ALTER TABLE entity_vendor_ids ADD CONSTRAINT entity_vendor_ids_pkey PRIMARY KEY (entity_type, entity_id, vendor);
ALTER TABLE entity_vendor_ids ADD CONSTRAINT entity_vendor_ids_type_vendor_id_unique UNIQUE (entity_type, vendor, vendor_id);
CREATE INDEX idx_entity_vendor_ids_type_entity ON entity_vendor_ids (entity_type, entity_id);

-- ---- odds_blaze_market_ids ----
ALTER TABLE odds_blaze_market_ids DROP CONSTRAINT odds_blaze_market_ids_entity_sportsbook_side_unique;
ALTER TABLE odds_blaze_market_ids DROP CONSTRAINT odds_blaze_market_ids_type_sportsbook_id_unique;
ALTER TABLE odds_blaze_market_ids DROP COLUMN entity_id;
ALTER TABLE odds_blaze_market_ids RENAME COLUMN old_entity_id TO entity_id;
ALTER TABLE odds_blaze_market_ids ADD CONSTRAINT odds_blaze_market_ids_entity_sportsbook_side_unique UNIQUE (entity_type, entity_id, sportsbook, side);
ALTER TABLE odds_blaze_market_ids ADD CONSTRAINT odds_blaze_market_ids_type_sportsbook_id_unique UNIQUE (entity_type, sportsbook, odds_blaze_id);

-- +goose StatementEnd
