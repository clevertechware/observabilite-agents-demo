package fr.clevertechware.demo.order;

import java.math.BigDecimal;
import java.util.Optional;
import org.springframework.jdbc.core.simple.JdbcClient;
import org.springframework.stereotype.Repository;

@Repository
class OrderRepository {

    private final JdbcClient jdbc;

    OrderRepository(JdbcClient jdbc) {
        this.jdbc = jdbc;
    }

    Order insert(String sku, int quantity, BigDecimal unitPrice, BigDecimal total) {
        long id = jdbc.sql("""
                        INSERT INTO orders (sku, quantity, unit_price, total)
                        VALUES (:sku, :quantity, :unitPrice, :total)
                        RETURNING id
                        """)
                .param("sku", sku)
                .param("quantity", quantity)
                .param("unitPrice", unitPrice)
                .param("total", total)
                .query(Long.class)
                .single();
        return new Order(id, sku, quantity, unitPrice, total);
    }

    Optional<Order> findById(long id) {
        return jdbc.sql("SELECT id, sku, quantity, unit_price, total FROM orders WHERE id = :id")
                .param("id", id)
                .query((rs, rowNum) -> new Order(
                        rs.getLong("id"),
                        rs.getString("sku"),
                        rs.getInt("quantity"),
                        rs.getBigDecimal("unit_price"),
                        rs.getBigDecimal("total")))
                .optional();
    }
}
