-- The enum held only families, so VS Code forks logged as vscode. Rewrites
-- the table under an exclusive lock.
ALTER TABLE connection_logs
	ALTER COLUMN type TYPE text;

COMMENT ON COLUMN connection_logs.type IS 'The app that connected, such as cursor, or a web connection type. Older rows hold the app family.';

DROP TYPE connection_type;
