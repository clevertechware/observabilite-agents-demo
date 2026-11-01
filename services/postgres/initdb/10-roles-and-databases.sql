-- La base `inventory` et le rôle `inventory` sont créés par l'entrypoint
-- (POSTGRES_USER / POSTGRES_DB). Il reste à créer le rôle et la base `orders`.
-- Ce script s'exécute contre POSTGRES_DB en tant que superuser.
CREATE ROLE orders LOGIN PASSWORD 'orders';
CREATE DATABASE orders OWNER orders;
