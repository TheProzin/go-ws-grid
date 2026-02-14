let WS_GRID_ATUAL = null;
const QTD_PIXELS = 25;

const criarWsUsuario = function () {
    $("#p_aviso").val("");
    const input_usuario_nome = $("#usuario_nome")
    const usuario_nome = input_usuario_nome.val();
    if (usuario_nome.length < 3) {
        $("#p_aviso").html("Pelo menos 3 caracteres");
        return;
    }

    getTokenWs();
}

const getTokenWs = function () {
    $.ajax({
        url: "http://localhost:9000/getTokenWsGrid",
        method: 'POST',
        contentType: 'application/json',
        dataType: 'json',
        data: JSON.stringify({
            nome_usuario: $("#usuario_nome").val(),
            id_usuario: self.crypto.randomUUID()
        }),
        success: function (retorno) {
            fecharConexaoWs();
            wsGrid(retorno);
        },
        error: function (xhr, status, error) {
            console.error("ERRO DETALHADO:", {
                status: status,
                error: error,
                response: xhr.responseText,
                statusCode: xhr.status
            });
            alert("Erro na requisição: " + error);
        }
    });
}

const fecharConexaoWs = function () {
    if (WS_GRID_ATUAL && WS_GRID_ATUAL.readyState === WebSocket.OPEN) {
        WS_GRID_ATUAL.close();
        WS_GRID_ATUAL = null;
        console.log("Conexão WebSocket anterior fechada");
    }
}

const wsGrid = function (retorno) {
    if (!retorno || !retorno.token) {
        console.error("Token não recebido");
        return;
    }

    const token = retorno.token;
    const urlWS = `ws://localhost:9000/wsGrid?otp=` + token;

    const socket = new WebSocket(urlWS);

    socket.onopen = function () {
        console.log("WebSocket conectado com sucesso!");
        WS_GRID_ATUAL = socket;

        $("#div_cadastro_usuario").hide();
        iniciaGrid();
        $("#div_cor").show();
    };

    socket.onmessage = function (event) {
        try {
            const jsonString = event.data.replace(/\u0000/g, '').trim();
            const json_grid = JSON.parse(jsonString);

            if (json_grid) {
                montarGrid(json_grid);
            }
        } catch (e) {
            console.error("Erro ao processar mensagem:", e);
        }
    };

    socket.onerror = function (error) {
        console.error("Erro no WebSocket:", error);
    };

    socket.onclose = function (event) {
        console.log("WebSocket fechado:", event.code, event.reason);
        if (WS_GRID_ATUAL === socket) {
            WS_GRID_ATUAL = null;
        }
    };
}

const enviarCor = function () {

    if (!WS_GRID_ATUAL) {
        console.error("Nenhuma conexão WebSocket ativa.");
        return;
    }

    if (WS_GRID_ATUAL.readyState === WebSocket.OPEN) {
        const cor = $("#cor").val();
        WS_GRID_ATUAL.send(JSON.stringify(cor));
    } else {
        console.error("WebSocket não está aberto. Estado:", WS_GRID_ATUAL.readyState);
    }
};

const iniciaGrid = function () {
    const grid = $("#pixel_grid");
    grid.empty();
    
    const { colunas, linhas } = encontrarMelhorGrid(QTD_PIXELS);
    
    grid.css({
        display: 'grid',
        gridTemplateColumns: `repeat(${colunas}, 1fr)`,
        gap: '4px',
        width: 'fit-content',
        margin: '0 auto'
    });

    for (let i = 0; i < QTD_PIXELS; i++) {
        const numero_exibicao = i + 1;
        const pixel = $("<div>")
            .addClass("pixel")
            .attr("data-indice", i)
            .html(numero_exibicao)
            .css({
                width: "50px",
                height: "50px",
                backgroundColor: "aqua",
                color: "#ccc",
                border: "1px solid #ccc",
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center'
            });

        grid.append(pixel);
    }
}

const encontrarMelhorGrid = function (total) {
    let melhorColunas = total;
    let melhorDiferenca = Infinity;
    
    for (let colunas = 1; colunas <= Math.sqrt(total) * 2; colunas++) {
        const linhas = Math.ceil(total / colunas);
        const diferenca = Math.abs(colunas - linhas);
        
        if (diferenca < melhorDiferenca) {
            melhorDiferenca = diferenca;
            melhorColunas = colunas;
        }
    }
    
    return {
        colunas: melhorColunas,
        linhas: Math.ceil(total / melhorColunas)
    };
}

const montarGrid = function (json_grid) {
    const objeto = typeof json_grid === 'string' ? JSON.parse(json_grid) : json_grid

    $("#proximo_pixel").html(`Próximo Pixel: ${objeto.proximo_pixel}`);
    
    const grid = Object.keys(objeto.grid_cores).map(indice => ({
        indice: parseInt(indice),
        valor: objeto.grid_cores[indice]
    }))

    grid.forEach(item => {
        const pixel = $(`#pixel_grid div[data-indice="${item.indice}"]`);
        if (pixel.length) {
            pixel.css("backgroundColor", item.valor);
        }
    });
}