package fr.clevertechware.demo.order;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;

@Component
class HeapLeakFilter extends OncePerRequestFilter {

    private final HeapLeakFault fault;

    HeapLeakFilter(HeapLeakFault fault) {
        this.fault = fault;
    }

    @Override
    protected boolean shouldNotFilter(HttpServletRequest request) {
        return request.getRequestURI().equals("/healthz");
    }

    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain chain)
            throws ServletException, IOException {
        fault.onRequest();
        chain.doFilter(request, response);
    }
}
