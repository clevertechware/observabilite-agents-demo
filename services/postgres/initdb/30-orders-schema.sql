-- Schéma d'order-service, appliqué sur la base `orders` avec le rôle `orders`
-- comme propriétaire des tables (comme le fait Spring au démarrage en k8s).
\connect orders
SET ROLE orders;
\i /opt/initdb/orders-schema.sql
