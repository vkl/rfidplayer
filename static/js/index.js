function updateCardTable(cards) {
    const cardsTable = document.getElementById("cards");
    const rows = cardsTable.querySelectorAll("tr:not(:first-child)");
    rows.forEach((row) => {
        cardsTable.removeChild(row);
    })
    Object.entries(cards).forEach(([id, card]) => {
        const newRow = document.createElement("tr")
        links = ""
        for (entry of card.media_links) {
            links += entry.link+"; "+entry.content_type + "\n</br>"
        }
        newRow.innerHTML = `<td>
                <input id="`+id+`" type="checkbox"/></td>
                <td>`+id+`</td>
                <td>`+card.name+`</td>
                <td>`+card.chromecast+`</td>
                <td><div class="wrapper">`+links+`</div></td><td>
                <a class="play" onclick="playCard(this)" href="#">play</a>
                <a class="edit" onclick="editCard(this)" href="#">edit</a></td>`;
        cardsTable.appendChild(newRow);
    });
}

async function editCard(element) {
    const divCardData = document.getElementById("editcard");
    divCardData.classList.toggle("hidden");
    const cardId = element
        .closest("tr")
        .querySelector("input").id;
    const cardName = element
        .closest("tr")
        .querySelector("td:nth-child(3)").textContent
    const cardChromecast = element
        .closest("tr")
        .querySelector("td:nth-child(4)").textContent
    const cardMediaLinks = element
        .closest("tr")
        .querySelector("td:nth-child(5)").textContent
    divCardData.querySelector("#media_links").value = cardMediaLinks
    divCardData.querySelector("#id").value = cardId
    divCardData.querySelector("#chromecast").value = cardChromecast
    divCardData.querySelector("#name").value = cardName
}

async function playCard(element) {
    try {
        const cardId = element
            .closest("tr")
            .querySelector("input").id;
        const castName = element
            .closest("tr")
            .querySelector("td:nth-child(4)").textContent;
        const response = await fetch("/api/cards/"+cardId, {
            method: "POST",
            headers: {
            "Content-Type": "application/json",
            },
            body: JSON.stringify({})
        });
    } catch (error) {
        console.error('Error:', error);
    }
}

async function castControl(element) {
    const action = element.className;
    const volume = element
        .closest("tr")
        .querySelector("#volume").value;
    const castName = element
        .closest("tr")
        .querySelector("td:first-child").textContent;
    const payload = {
        "action": action,
        "volume": parseFloat(volume)
    };
    const response = await fetch("/api/casts/"+castName, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload)
    });
}

function updateCastTable(casts) {
    const castTable = document.getElementById("casts");
    const selectCast = document.getElementById("chromecast")
    const rows = castTable.querySelectorAll("tr");
    rows.forEach((row) => {
        castTable.removeChild(row);
    })
    const options = selectCast.querySelectorAll("option")
    options.forEach((option) => {
        selectCast.removeChild(option);
    })
    for (const cast of casts) {
        const newRow = document.createElement("tr")
        const newOption = document.createElement("option")
        newRow.innerHTML = `<td id="`+cast.name+`">`+cast.name+`</td>
            <td>`+cast.status+` `+cast.media_status+` `+cast.media_data+`</td>
            <td><input type="number" id="volume" step="0.05" min="0" max="1" value=`+cast.volume.toFixed(2)+`>
            <a class="setvolume" onclick="castControl(this)" href="#">set</a></td>
            <td>
                <a class="play" onclick="castControl(this)" href="#">play</a>
                <a class="pause" onclick="castControl(this)" href="#">pause</a>
                <a class="next" onclick="castControl(this)" href="#">next</a>
                <a class="prev" onclick="castControl(this)" href="#">prev</a>
                <a class="stop" onclick="castControl(this)" href="#">stop</a>
            </td>`;
        newOption.value = newOption.textContent = cast.name;
        castTable.appendChild(newRow);
        selectCast.appendChild(newOption)
    }
}

async function getCards() {
    try {
        const response = await fetch('/api/cards');
        if (!response.ok) {
            throw new Error('Network response was not ok');
        }
        const cards = await response.json();
        updateCardTable(cards)
    } catch (error) {
        console.error('Error:', error);
    }
}

async function addCard(event) {
    const divCardData = document.getElementById("editcard");
    payload = {};
    for (const input of divCardData.querySelectorAll("input, textarea, select")) {
        if (input.tagName == "TEXTAREA") {
            payload[input.id] = []
            for (media_link of input.value.trim().split("\n")) {
                tokens = media_link.split(";")
                payload[input.id].push({"link": tokens[0], "content_type": tokens[1]});
            }
        } else {
            payload[input.id] = input.value;
        }
    }
    console.log(JSON.stringify(payload));
    try {
        const response = await fetch("/api/cards", {
            method: "POST",
            headers: {
            "Content-Type": "application/json",
            },
            body: JSON.stringify(payload)
        });
        const cards = await response.json();
        cleanEditCard();
        updateCardTable(cards);
    } catch (error) {
        console.error('Error:', error);
    }
    document.getElementById("editcard").classList.toggle("hidden");
}

async function delCard(event) {
    const cardsData = document.getElementById("cards")
        .querySelectorAll('input[type="checkbox"]:checked');
    for (const card of cardsData) {
        const response = await fetch("/api/cards/" + card.id, {
            method: "DELETE",
            headers: {
                "Content-Type": "application/json",
            }
        });
        const cards = await response.json();
        updateCardTable(cards);
    }
}

function cleanEditCard() {
    const editCardDiv = document.getElementById("editcard");
    editCardDiv.querySelectorAll("input, select, textarea").forEach((chElement) => {
        if (chElement.tagName == "SELECT") {
            return;
        }
        chElement.value = '';
    })
}

async function updateCasts() {
    payload = {"action": "refresh"};
    const response = await fetch("/api/casts", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload)
    });
    setInterval(getCasts, 5000);
}

async function getCasts() {
    try {
        const response = await fetch('/api/casts');
        if (!response.ok) {
            throw new Error('Network response was not ok');
        }
        const casts = await response.json();
        updateCastTable(casts)
    } catch (error) {
        console.error('Error:', error);
    }
}

window.addEventListener("load", async (event) => {
    await getCards();
    await updateCasts();
    const addCardBtn = document.getElementById("addcard");
    const delCardBtn = document.getElementById("delcard");
    const updateCastBtn = document.getElementById("updatecc");
    const updateCardsListBtn = document.getElementById("updatecards");

    document.getElementById("closeBtn").addEventListener("click", () =>{
        document.getElementById("editcard").classList.toggle("hidden");
    })

    document.getElementById("newcard").addEventListener("click", () => {
        document.getElementById("editcard").classList.toggle("hidden");
    })


    addCardBtn.addEventListener("click", addCard);
    delCardBtn.addEventListener("click", delCard);
    updateCastBtn.addEventListener("click", updateCasts);
    updateCardsListBtn.addEventListener("click", getCards);
});
