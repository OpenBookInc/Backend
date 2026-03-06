-- +goose Up

-- =============================================================================
-- individual_statuses: drop id, use individual_id as PK
-- =============================================================================
ALTER TABLE individual_statuses DROP CONSTRAINT individual_statuses_pkey;
ALTER TABLE individual_statuses DROP CONSTRAINT individual_statuses_individual_id_unique;
ALTER TABLE individual_statuses DROP COLUMN id;
ALTER TABLE individual_statuses ADD CONSTRAINT individual_statuses_pkey PRIMARY KEY (individual_id);

-- =============================================================================
-- nfl_box_scores: drop id, use (game_id, individual_id) as composite PK
-- =============================================================================
ALTER TABLE nfl_box_scores DROP CONSTRAINT nfl_box_scores_pkey;
ALTER TABLE nfl_box_scores DROP CONSTRAINT nfl_box_scores_game_id_individual_id_key;
ALTER TABLE nfl_box_scores DROP COLUMN id;
ALTER TABLE nfl_box_scores ADD CONSTRAINT nfl_box_scores_pkey PRIMARY KEY (game_id, individual_id);

-- =============================================================================
-- nba_box_scores: drop id, use (game_id, individual_id) as composite PK
-- =============================================================================
ALTER TABLE nba_box_scores DROP CONSTRAINT nba_box_scores_pkey;
ALTER TABLE nba_box_scores DROP CONSTRAINT nba_box_scores_game_id_individual_id_key;
ALTER TABLE nba_box_scores DROP COLUMN id;
ALTER TABLE nba_box_scores ADD CONSTRAINT nba_box_scores_pkey PRIMARY KEY (game_id, individual_id);

-- =============================================================================
-- rosters: drop id, use team_id as PK
-- =============================================================================
ALTER TABLE rosters DROP CONSTRAINT rosters_pkey;
ALTER TABLE rosters DROP CONSTRAINT rosters_team_id_unique;
ALTER TABLE rosters DROP COLUMN id;
ALTER TABLE rosters ADD CONSTRAINT rosters_pkey PRIMARY KEY (team_id);

-- +goose Down

-- =============================================================================
-- rosters: restore id column
-- =============================================================================
ALTER TABLE rosters DROP CONSTRAINT rosters_pkey;
ALTER TABLE rosters ADD COLUMN id serial;
ALTER TABLE rosters ADD CONSTRAINT rosters_pkey PRIMARY KEY (id);
ALTER TABLE rosters ADD CONSTRAINT rosters_team_id_unique UNIQUE (team_id);

-- =============================================================================
-- nba_box_scores: restore id column
-- =============================================================================
ALTER TABLE nba_box_scores DROP CONSTRAINT nba_box_scores_pkey;
ALTER TABLE nba_box_scores ADD COLUMN id serial;
ALTER TABLE nba_box_scores ADD CONSTRAINT nba_box_scores_pkey PRIMARY KEY (id);
ALTER TABLE nba_box_scores ADD CONSTRAINT nba_box_scores_game_id_individual_id_key UNIQUE (game_id, individual_id);

-- =============================================================================
-- nfl_box_scores: restore id column
-- =============================================================================
ALTER TABLE nfl_box_scores DROP CONSTRAINT nfl_box_scores_pkey;
ALTER TABLE nfl_box_scores ADD COLUMN id serial;
ALTER TABLE nfl_box_scores ADD CONSTRAINT nfl_box_scores_pkey PRIMARY KEY (id);
ALTER TABLE nfl_box_scores ADD CONSTRAINT nfl_box_scores_game_id_individual_id_key UNIQUE (game_id, individual_id);

-- =============================================================================
-- individual_statuses: restore id column
-- =============================================================================
ALTER TABLE individual_statuses DROP CONSTRAINT individual_statuses_pkey;
ALTER TABLE individual_statuses ADD COLUMN id serial;
ALTER TABLE individual_statuses ADD CONSTRAINT individual_statuses_pkey PRIMARY KEY (id);
ALTER TABLE individual_statuses ADD CONSTRAINT individual_statuses_individual_id_unique UNIQUE (individual_id);
