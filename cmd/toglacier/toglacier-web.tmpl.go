package main

var webHomepageTemplate = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
    <title>toglacier</title>

    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0-alpha.6/css/bootstrap.min.css" integrity="sha384-rwoIResjU2yc3z8GV/NPeZWAv56rSmLldC3R/AZzGRnGxQQKnKkoFVhFQhNUwEyJ" crossorigin="anonymous">
  </head>
  <body>
    <nav class="navbar navbar-inverse bg-primary">
      <a class="navbar-brand" href="#">toglacier</a>
    </nav>
    
    <div class="container-fluid">
      <div class="row">
        <div class="col">
          <div class="alert alert-danger alert-dismissible fade show mt-2" role="alert">
            <button type="button" class="close" data-dismiss="alert" aria-label="Close">
              <span aria-hidden="true">&times;</span>
            </button>
            <p></p>
          </div>
        </div>
      </div>

      <div class="row">
        <div class="col">
          <ul class="nav nav-tabs mt-2" role="tablist">
            <li class="nav-item">
              <a class="nav-link active" data-toggle="tab" href="#backups" role="tab">Backups</a>
            </li>
            <li class="nav-item">
              <a class="nav-link" data-toggle="tab" href="#settings" role="tab">Settings</a>
            </li>
          </ul>
        </div>
      </div>

      <div class="row">
        <div class="col">
          <div class="tab-content">
            <div class="tab-pane fade show active" id="backups" role="tabpanel">
              <table class="table">
                <thead>
                  <tr>
                    <th>#</th>
                    <th>Date</th>
                  </tr>
                </thead>
                <tbody id="backups"></tbody>
              </table>
            </div>
            <div class="tab-pane fade" id="settings" role="tabpanel">
      
            </div>
          </div>
        </div>
      </div>
    </div>

    <script src="https://code.jquery.com/jquery-3.2.1.min.js" integrity="sha256-hwg4gsxgFZhOsEEamdOYGBf13FyQuiTwlAQgxVSNgt4=" crossorigin="anonymous"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/tether/1.4.0/js/tether.min.js" integrity="sha384-DztdAPBWPRXSA/3eYEEUWrWCy7G5KFbe8fFjk5JAIxUYHKkDx6Qin1DkWx51bBrb" crossorigin="anonymous"></script>
    <script src="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0-alpha.6/js/bootstrap.min.js" integrity="sha384-vBWWzlZJ8ea9aCX4pEW3rVHjgjt7zpkNpZk+02D9phzyeVkE+jo0ieGizqPLForn" crossorigin="anonymous"></script>

    <script type="text/javascript">
      function backups() {
        $.ajax("/backups")

          .done(function(backups, textStatus, jqXHR) {
            $("#backups").empty();
            $.each(JSON.parse(backups), function(index, backup) {
              $("#backups").append(
                $("<tr>").append(
                  $("<th>")
                    .attr("scope", "row")
                    .text(index),
                  $("<td>").text(backup.Backup.CreatedAt)
                )
              );
            });
          })

          .fail(function(jqXHR, textStatus, errorThrown) {
            $(".alert-danger > p").text("Error loading backups");
            $(".alert").alert();
          })

          .always(function() {
            $(".alert-danger > p").text("");
            $(".alert").alert("close");
            setTimeout(backups, 5000);
          });
      }

      $(function() {
        backups();
      });
    </script>
  </body>
</html>
`
