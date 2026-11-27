-- Remove historical bulk-dataset responses (nflverse and nfelo) from the raw
-- HTTP archive; both sources upsert directly into parsed tables.
DELETE FROM api_responses
WHERE source IN ('nflverse', 'nfelo');

DELETE FROM api_bodies
WHERE sha256 NOT IN (SELECT body_sha256 FROM api_responses);
