-- Path of the library config (dws_debugger.json); NULL means <root_path>/dws_debugger.json.
ALTER TABLE projects ADD COLUMN config_path TEXT;
