-- Remap runner profiles that point at model ids their providers have retired (as of
-- 2026-09) to the current replacements. Data only — model/effort are free-form columns,
-- no DDL needed; profiles on live models are left as chosen. The ids are unambiguous,
-- so the mapping applies to every runner (prefixed ones only exist under opencode) —
-- except profiles with their own apiBaseURL: behind a gateway (LiteLLM, Azure) the
-- model is the gateway's alias, which may well still exist.
UPDATE "runnerProfiles" p SET "model" = m."new"
FROM (VALUES
	-- OpenAI: the gpt-5-codex / gpt-5.1-codex* / gpt-5.2-codex family, shut down 2026-07-23.
	('gpt-5-codex', 'gpt-6-sol'),
	('gpt-5.1-codex', 'gpt-6-sol'),
	('gpt-5.1-codex-max', 'gpt-6-sol'),
	('gpt-5.1-codex-mini', 'gpt-6-sol'),
	('gpt-5.2-codex', 'gpt-6-sol'),
	-- DeepSeek: V4 Flash and the chat/reasoner aliases are retired, served as deepseek-flash.
	('deepseek-v4-flash', 'deepseek-flash'),
	('deepseek-chat', 'deepseek-flash'),
	('deepseek-reasoner', 'deepseek-flash'),
	('deepseek/deepseek-v4-flash', 'deepseek/deepseek-flash'),
	-- Anthropic: Opus 4.1 retired 2026-08-05, Opus 4 / Sonnet 4 retired 2026-06-15.
	('claude-opus-4-1', 'claude-opus-5-5'),
	('claude-opus-4-1-20250805', 'claude-opus-5-5'),
	('claude-opus-4-0', 'claude-opus-5-5'),
	('claude-opus-4-20250514', 'claude-opus-5-5'),
	('claude-sonnet-4-0', 'claude-sonnet-5'),
	('claude-sonnet-4-20250514', 'claude-sonnet-5'),
	('anthropic/claude-opus-4-1', 'anthropic/claude-opus-5-5'),
	('anthropic/claude-opus-4-1-20250805', 'anthropic/claude-opus-5-5'),
	('anthropic/claude-opus-4-0', 'anthropic/claude-opus-5-5'),
	('anthropic/claude-opus-4-20250514', 'anthropic/claude-opus-5-5'),
	('anthropic/claude-sonnet-4-0', 'anthropic/claude-sonnet-5'),
	('anthropic/claude-sonnet-4-20250514', 'anthropic/claude-sonnet-5')
) AS m("old", "new")
WHERE p."model" = m."old"
	AND coalesce(p."apiBaseURL", '') = '';
