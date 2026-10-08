package fr.clevertechware.demo.order;

import static com.github.tomakehurst.wiremock.client.WireMock.aResponse;
import static com.github.tomakehurst.wiremock.client.WireMock.equalToJson;
import static com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor;
import static com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo;
import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.github.tomakehurst.wiremock.client.WireMock;
import com.github.tomakehurst.wiremock.core.WireMockConfiguration;
import com.github.tomakehurst.wiremock.http.Fault;
import com.github.tomakehurst.wiremock.junit5.WireMockExtension;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.RegisterExtension;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.testcontainers.service.connection.ServiceConnection;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.http.MediaType;
import org.springframework.jdbc.core.simple.JdbcClient;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.postgresql.PostgreSQLContainer;

@SpringBootTest
@AutoConfigureMockMvc
@Testcontainers
class OrderApiTest {

    private static final String ORDER_BODY = """
            {"sku": "SKU-1", "quantity": 2}
            """;

    @Container
    @ServiceConnection
    static PostgreSQLContainer postgres = new PostgreSQLContainer("postgres:18.6-trixie");

    @RegisterExtension
    static WireMockExtension inventory = WireMockExtension.newInstance()
            .options(WireMockConfiguration.wireMockConfig().dynamicPort().http2PlainDisabled(true))
            .build();

    @DynamicPropertySource
    static void inventoryProperties(DynamicPropertyRegistry registry) {
        registry.add("inventory.base-url", inventory::baseUrl);
        registry.add("spring.http.client.read-timeout", () -> "500ms");
    }

    @Autowired
    MockMvc mockMvc;

    @Autowired
    JdbcClient jdbc;

    @BeforeEach
    void emptyOrdersTable() {
        jdbc.sql("TRUNCATE TABLE orders").update();
    }

    @Test
    void creates_order_with_unit_price_returned_by_inventory() throws Exception {
        stubReservation(200, """
                {"sku": "SKU-1", "quantity": 2, "unitPrice": 10.00, "total": 20.00}
                """);

        mockMvc.perform(post("/orders").contentType(MediaType.APPLICATION_JSON).content(ORDER_BODY))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.id").isNumber())
                .andExpect(jsonPath("$.sku").value("SKU-1"))
                .andExpect(jsonPath("$.quantity").value(2))
                .andExpect(jsonPath("$.unitPrice").value(10.00))
                .andExpect(jsonPath("$.total").value(20.00));
    }

    @Test
    void stores_total_returned_by_inventory_instead_of_recomputing_it() throws Exception {
        stubReservation(200, """
                {"sku": "SKU-1", "quantity": 2, "unitPrice": 10.00, "total": 18.50}
                """);

        mockMvc.perform(post("/orders").contentType(MediaType.APPLICATION_JSON).content(ORDER_BODY))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.total").value(18.50));

        Integer rows = jdbc.sql("SELECT count(*) FROM orders WHERE total = 18.50").query(Integer.class).single();
        assertThat(rows).isEqualTo(1);
    }

    @Test
    void sends_sku_and_quantity_to_inventory_reservations_endpoint() throws Exception {
        stubReservation(200, """
                {"sku": "SKU-1", "quantity": 2, "unitPrice": 10.00, "total": 20.00}
                """);

        mockMvc.perform(post("/orders").contentType(MediaType.APPLICATION_JSON).content(ORDER_BODY))
                .andExpect(status().isCreated());

        inventory.verify(postRequestedFor(urlEqualTo("/reservations"))
                .withRequestBody(equalToJson(ORDER_BODY)));
    }

    @Test
    void persists_created_order_in_orders_table() throws Exception {
        stubReservation(200, """
                {"sku": "SKU-1", "quantity": 2, "unitPrice": 10.00, "total": 20.00}
                """);

        mockMvc.perform(post("/orders").contentType(MediaType.APPLICATION_JSON).content(ORDER_BODY))
                .andExpect(status().isCreated());

        Integer rows = jdbc.sql("SELECT count(*) FROM orders WHERE sku = 'SKU-1' AND total = 20.00")
                .query(Integer.class)
                .single();
        assertThat(rows).isEqualTo(1);
    }

    @Test
    void returns_created_order_by_id() throws Exception {
        stubReservation(200, """
                {"sku": "SKU-1", "quantity": 2, "unitPrice": 10.00, "total": 20.00}
                """);
        MvcResult created = mockMvc.perform(post("/orders").contentType(MediaType.APPLICATION_JSON).content(ORDER_BODY))
                .andExpect(status().isCreated())
                .andReturn();

        mockMvc.perform(get(created.getResponse().getHeader("Location")))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.sku").value("SKU-1"))
                .andExpect(jsonPath("$.total").value(20.00));
    }

    @Test
    void returns_404_when_order_does_not_exist() throws Exception {
        mockMvc.perform(get("/orders/999999")).andExpect(status().isNotFound());
    }

    @Test
    void propagates_409_when_inventory_reports_insufficient_stock() throws Exception {
        stubReservation(409, """
                {"error": "insufficient stock"}
                """);

        mockMvc.perform(post("/orders").contentType(MediaType.APPLICATION_JSON).content(ORDER_BODY))
                .andExpect(status().isConflict());
        assertThat(orderCount()).isZero();
    }

    @Test
    void returns_404_when_inventory_reports_unknown_sku() throws Exception {
        stubReservation(404, """
                {"error": "unknown sku"}
                """);

        mockMvc.perform(post("/orders").contentType(MediaType.APPLICATION_JSON).content(ORDER_BODY))
                .andExpect(status().isNotFound());
        assertThat(orderCount()).isZero();
    }

    @ParameterizedTest
    @ValueSource(ints = {500, 503})
    void returns_502_when_inventory_answers_with_unexpected_error_status(int inventoryStatus) throws Exception {
        stubReservation(inventoryStatus, "{}");

        mockMvc.perform(post("/orders").contentType(MediaType.APPLICATION_JSON).content(ORDER_BODY))
                .andExpect(status().isBadGateway());
        assertThat(orderCount()).isZero();
    }

    @Test
    void returns_502_when_inventory_reservation_has_no_total() throws Exception {
        stubReservation(200, """
                {"sku": "SKU-1", "quantity": 2, "unitPrice": 10.00}
                """);

        mockMvc.perform(post("/orders").contentType(MediaType.APPLICATION_JSON).content(ORDER_BODY))
                .andExpect(status().isBadGateway());
        assertThat(orderCount()).isZero();
    }

    @Test
    void returns_502_when_inventory_connection_is_reset() throws Exception {
        inventory.stubFor(WireMock.post("/reservations")
                .willReturn(aResponse().withFault(Fault.CONNECTION_RESET_BY_PEER)));

        mockMvc.perform(post("/orders").contentType(MediaType.APPLICATION_JSON).content(ORDER_BODY))
                .andExpect(status().isBadGateway());
    }

    @Test
    void returns_502_when_inventory_exceeds_read_timeout() throws Exception {
        inventory.stubFor(WireMock.post("/reservations")
                .willReturn(aResponse().withStatus(200).withFixedDelay(2000)));

        mockMvc.perform(post("/orders").contentType(MediaType.APPLICATION_JSON).content(ORDER_BODY))
                .andExpect(status().isBadGateway());
    }

    @ParameterizedTest
    @ValueSource(strings = {
            """
            {"sku": "", "quantity": 2}
            """,
            """
            {"sku": "SKU-1", "quantity": 0}
            """,
            """
            {"quantity": 2}
            """})
    void rejects_invalid_order_request_with_400_without_calling_inventory(String body) throws Exception {
        mockMvc.perform(post("/orders").contentType(MediaType.APPLICATION_JSON).content(body))
                .andExpect(status().isBadRequest());

        inventory.verify(0, postRequestedFor(urlEqualTo("/reservations")));
    }

    @Test
    void healthz_answers_200() throws Exception {
        mockMvc.perform(get("/healthz")).andExpect(status().isOk());
    }

    private void stubReservation(int status, String body) {
        inventory.stubFor(WireMock.post("/reservations")
                .willReturn(aResponse()
                        .withStatus(status)
                        .withHeader("Content-Type", "application/json")
                        .withBody(body)));
    }

    private int orderCount() {
        return jdbc.sql("SELECT count(*) FROM orders").query(Integer.class).single();
    }
}
